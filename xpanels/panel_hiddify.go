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
	"log"
	"net/url"
	"os"
	"time"
)

const unreachableMsg = "You shouldn't see this warning!"

func NewHiddifyPanel(xs *XrayService, conf map[string]string) *HiddifyPanel {
	log := confPackage.NewLogger()
	name := "hiddify.com" // Note: hiddify only works with Emails having "@hiddify.com" postfix
	dbPath, _ := conf["db"]

	repo := SetupHiddifyRepo(dbPath)
	repo.migrate()

	fPath, _ := conf["subFile"]
	b, err := os.ReadFile(fPath)
	if err != nil {
		log.Fatal(err)
	}

	return &HiddifyPanel{
		name: name,
		repo: repo,
		xray: xs,
		log:  log,

		renovator: NewRenovatorFromFile(string(b)),
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

	renovator SubRenovator
}

func (panel *HiddifyPanel) Ping(_ context.Context, _ *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func (panel *HiddifyPanel) AddUser(ctx context.Context, cmd *pb.AddUserCmd) (*pb.Response, error) {
	panel.log.Info("Received AddUser cmd ->", cmd)
	//// 1- Add a user to panel's database
	err := panel.add2panel(cmd)
	if err != nil {
		// if this user already exists on db, return a successful response
		err2, ok := err.(sqlite3.Error)
		if ok && err2.Code == sqlite3.ErrConstraint {
			return &pb.Response{}, nil
		}
		panel.log.Error("failed to add user to hiddify-db:", err)
		return &pb.Response{}, status.Error(codes.Internal, err.Error())
	}
	// 2- Add a client to xray-core
	err = panel.add2xray(cmd)
	if err != nil {
		panel.log.Error("failed to add user to xray-core:", err)
		return &pb.Response{}, status.Error(codes.Aborted, err.Error())
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

	return panel.repo.InsertUser(cmd.Uuid, cmd.TUsername, expireTime, startDate, cmd.Package.Mode, lastOnline, cmd.Package.TrafficAllowed, cmd.Package.PackageDays)
}

func (panel *HiddifyPanel) add2xray(cmd *pb.AddUserCmd) error {
	var err error
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
		err = panel.xray.AddClient(x, name)
		if err != nil {
			nOfErrs++
		}
	}
	if nOfErrs > len(inboundNames)*2/3 {
		// failed for more than 2/3 of inbounds
		return fmt.Errorf("%d/%d inbounds failed - reason: %s", nOfErrs, len(inboundNames), err)
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
	newSubContent, err := panel.renovator.Renovate(nil, uid)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.SubContent{Content: newSubContent}, nil
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
