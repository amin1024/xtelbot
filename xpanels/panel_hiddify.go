package xpanels

import (
	"context"
	"fmt"
	confPackage "github.com/amin1024/xtelbot/conf"
	"github.com/amin1024/xtelbot/pb"
	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const unreachableMsg = "You shouldn't see this warning!"

func NewHiddifyPanel(xs *XrayService, conf map[string]string) *HiddifyPanel {
	log := confPackage.NewLogger()
	name := "hiddify.com" // Note: hiddify only works with Emails having "@hiddify.com" postfix
	dbPath, _ := conf["db"]

	repo := SetupHiddifyRepo(dbPath)
	repo.migrate()

	return &HiddifyPanel{
		name: name,
		repo: repo,
		xray: xs,
		log:  log,
	}
}

func SetupHiddifyRepo(dbPath string) *HiddifyPanelRepo {
	db, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	return &HiddifyPanelRepo{db: db}
}

// -----------------------------------------------------------------

// HiddifyPanel implements pb.XNodeGrpcServer and SubRenovator
type HiddifyPanel struct {
	pb.UnimplementedXNodeGrpcServer
	name string
	repo IHiddifyPanelRepo
	xray *XrayService
	log  *zap.SugaredLogger
}

func (panel *HiddifyPanel) Ping(_ context.Context, _ *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func (panel *HiddifyPanel) AddUser(ctx context.Context, cmd *pb.AddUserCmd) (*pb.Response, error) {
	panel.log.Info("Received AddUser cmd ->", cmd)
	//// 1- Add a user to panel's database
	if err := panel.add2panel(cmd); err != nil {
		panel.log.Error("failed to add user to hiddify-db:", err)
		return &pb.Response{}, status.Error(codes.Internal, err.Error())
	}
	// 2- Add a client to xray-core
	err := panel.add2xray(cmd)
	if err != nil {
		panel.log.Error("failed to add user to xray-core:", err)
		return &pb.Response{}, status.Error(codes.Aborted, fmt.Errorf("partially done: %w", err).Error())
	}
	panel.log.Infow("Successfully added user to inbounds", "~tid", cmd.Tid, "~username", cmd.TUsername)
	// 3- Return a Response to the bot
	return &pb.Response{}, nil
}

func (panel *HiddifyPanel) add2panel(cmd *pb.AddUserCmd) error {
	now := time.Now()
	t, err := time.Parse(time.RFC3339, cmd.Package.ExpireAt)
	if err != nil {
		return err
	}
	expireTime := t.Format("2006-01-02")
	lastOnline := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	startDate := now.Format("2006-01-02")

	err = panel.repo.InsertUser(cmd.Uuid, cmd.TUsername, expireTime, startDate, cmd.Package.Mode, lastOnline, cmd.Package.TrafficAllowed, cmd.Package.PackageDays)
	// ignore if user already exists
	if err != nil {
		err, ok := err.(sqlite3.Error)
		if !ok || err.Code != sqlite3.ErrConstraint {
			return err
		}
	}
	return nil
}

func (panel *HiddifyPanel) add2xray(cmd *pb.AddUserCmd) error {
	x := XClient{
		Uuid:    cmd.Uuid,
		Email:   cmd.Uuid + "@" + panel.name,
		Level:   0,
		AlterId: 0,
	}
	// Get all available inbounds
	inboundNames, err := panel.getInboundNames()
	if err != nil {
		return err
	}
	nOfErrs := 0
	for _, name := range inboundNames {
		err := panel.xray.AddClient(x, name)
		if err != nil {
			nOfErrs++
		}
	}
	if nOfErrs > len(inboundNames)*2/3 {
		// failed for more than 2/3 of inbounds
		return fmt.Errorf("%d failed out of %d", nOfErrs, len(inboundNames))
	}

	return nil
}

func (panel *HiddifyPanel) getInboundNames() ([]string, error) {
	var names []string
	stats, err := panel.xray.GetInboundStats()
	if err != nil {
		return names, fmt.Errorf("cannot extract inbound names: %w", err)
	}
	for name, _ := range stats {
		if name == "api" {
			continue
		}
		names = append(names, name)
	}
	panel.log.Info("extracted inbound names:", names)
	return names, nil
}

func (panel *HiddifyPanel) generateSubLinks(uid string) ([]string, error) {
	var links []string

	domains, err := panel.repo.GetDomains()
	if err != nil || len(domains) < 1 {
		return links, fmt.Errorf("[db] unable to query domain names - %w", err)
	}

	confs, err := panel.repo.GetStrConfig()
	if err != nil {
		return links, fmt.Errorf("[db] unable to query hiddify config key-values - %w", err)
	}
	proxyPath := confs["proxy_path"]

	for _, domain := range domains {
		s, _ := url.JoinPath("http://", domain, proxyPath, uid, "all.txt")
		s = s + "?mode=new"
		links = append(links, s)
	}
	return links, nil
}

func (panel *HiddifyPanel) GetSub(ctx context.Context, uInfo *pb.UserInfoReq) (*pb.SubContent, error) {
	uid := uInfo.GetUuid()
	// Check if uuid does exist in this panel
	if _, err := panel.repo.GetUser(uid); err != nil {
		panel.log.Errorw("[db] GetUser error", "uuid", uid, "detail", err)
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid UUID: %s", uid))
	}
	links, err := panel.generateSubLinks(uid)
	if err != nil {
		panel.log.Error(err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	var response *http.Response
	// Fetch the first successful uri and ignore others
	for _, uri := range links {
		response, err = http.Get(uri) // TODO: add timeout context
		if err != nil || response.StatusCode != http.StatusOK {
			continue
		}
		break
	}
	if response == nil { // All URIs failed
		panel.log.Errorw("failed to fetch sub-content from sub-links", "user-uuid", uid, "links", links)
		return nil, status.Error(codes.Internal, "failed to fetch sub-links")
	}
	defer func() {
		if response.Body != nil {
			response.Body.Close()
		}
	}()
	newSubContent, err := panel.Renovate(response.Body)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.SubContent{Content: newSubContent}, nil
}

func (panel *HiddifyPanel) Renovate(subContent io.Reader) (string, error) {
	var result []string

	data, err := io.ReadAll(subContent)
	if err != nil {
		return "", err
	}
	// Parse content assuming every V2ray config is being seperated by '\n'
	lines := strings.Split(string(data), "\n")
	if len(lines) < 10 { // Hack around it, todo
		return "", fmt.Errorf("is a sub content with %d len valid", len(data))
	}

	groupSpecs, err := panel.repo.GetGroupedRules()
	if err != nil {
		panel.log.Error(err)
		return "", err
	}

	// renovate every v2ray config found in the sub
	for _, line := range lines {
		if len(line) < 10 {
			continue
		}
		v2rayUri := strings.TrimSpace(line)
		if strings.HasPrefix(v2rayUri, "#") {
			result = append(result, line)
			continue
		}
		split := strings.Split(v2rayUri, "#")
		if len(split) != 2 {
			panel.log.Warnw(unreachableMsg, "v2rayUri", v2rayUri)
			continue
		}
		rules, ok := groupSpecs[split[1]] // split[1] gives us the #remark in uri
		if ok {
			v2rayUri = panel.renovateV2rayConfig(v2rayUri, rules)
		}
		result = append(result, v2rayUri)
	}
	if len(result) == 0 {
		return "", fmt.Errorf("sub-content without any valid v2ray config")
	}

	return strings.Join(result, "\n"), nil
}

// renovateV2rayConfig receives a single v2ray uri and modify it according to the rules.
// Example uri vless://UUID@SERVER:PORT?security=tls&sni=SNI&type=grpc&serviceName=GRPC-NAME&#REMARK
// We want to replace the #REMARK and :PORT, Therefore, two rules needed to be passed to this method.
func (panel *HiddifyPanel) renovateV2rayConfig(v2rayUri string, specs []RenovateRule) string {
	for _, spec := range specs {
		if spec.Ignore {
			return "# IGNORED #"
		}
		v2rayUri = strings.ReplaceAll(v2rayUri, spec.OldValue, spec.NewValue)
	}
	return v2rayUri
}

func (panel *HiddifyPanel) GetUserInfo(ctx context.Context, uInfo *pb.UserInfoReq) (*pb.UserInfo, error) {
	uid := uInfo.GetUuid()
	// Check if uuid does exist in this panel
	user, err := panel.repo.GetUser(uid)
	if err != nil {
		panel.log.Errorw("[db] GetUser error", "uuid", uid, "detail", err)
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid UUID: %s", uid))
	}
	return &pb.UserInfo{
		Uuid:         user.Uuid,
		Name:         user.Name,
		LastOnline:   user.LastOnline,
		UsageLimit:   user.UsageLimitGB,
		CurrentUsage: user.CurrentUsageGB,
	}, nil
}

func (panel *HiddifyPanel) UpgradeUserPackage(ctx context.Context, cmd *pb.AddPackageCmd) (*pb.Response, error) {
	uid := cmd.GetUuid()
	user, err := panel.repo.GetUser(uid)
	if err != nil {
		panel.log.Errorw("[db] GetUser error", "uuid", uid, "detail", err)
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid UUID: %s", uid))
	}
	p := cmd.Package
	t, err := time.Parse(time.RFC3339, p.ExpireAt)
	if err != nil {
		return &pb.Response{}, status.Error(codes.Internal, err.Error())
	}

	expireTime := t.Format("2006-01-02")
	startDate := time.Now().Format("2006-01-02")
	trafficAllowed := p.TrafficAllowed * 85 / 100
	if err := panel.repo.UpdateUserPackage(user.Uuid, expireTime, startDate, p.Mode, trafficAllowed, p.PackageDays); err != nil {
		return &pb.Response{}, status.Error(codes.Internal, err.Error())
	}
	panel.log.Infow("successfully upgraded user on panel", "userUUID", uid, "duration", p.PackageDays, "trafficAllowed", trafficAllowed)
	return &pb.Response{
		Msg: "Done",
	}, nil
}
