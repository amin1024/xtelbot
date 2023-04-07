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
	"time"
)

func NewHiddifyPanel(xs *XrayService, conf map[string]string) *HiddifyPanel {
	log := confPackage.NewLogger()
	name, ok := conf["name"]
	dbPath, _ := conf["db"]
	if !ok {
		name = "abc" // TODO: generate name
	}
	db, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	return &HiddifyPanel{
		// TODO: Get this one from config
		name: name,
		db:   db,
		xray: xs,
		log:  log,
	}
}

// -----------------------------------------------------------------

type HiddifyPanel struct {
	pb.UnimplementedXNodeGrpcServer
	name string
	db   *sqlx.DB
	xray *XrayService
	log  *zap.SugaredLogger
}

func (panel *HiddifyPanel) Ping(_ context.Context, _ *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func (panel *HiddifyPanel) AddUser(ctx context.Context, cmd *pb.AddUserCmd) (*pb.Response, error) {
	//// 1- Add a user to panel's database
	if err := panel.add2panel(cmd); err != nil {
		return &pb.Response{}, status.Error(codes.Internal, err.Error())
	}
	// 2- Add a client to xray-core
	err := panel.add2xray(cmd)
	if err != nil {
		return &pb.Response{}, status.Error(codes.Aborted, fmt.Errorf("partially done: %w", err).Error())
	}
	// 3- Return a Response to the bot
	return &pb.Response{}, nil
}

func (panel *HiddifyPanel) add2panel(cmd *pb.AddUserCmd) error {
	q := `INSERT INTO
	user(uuid, name, last_online, expiry_time, usage_limit_GB, package_days, mode, start_date, current_usage_GB)
	values(?, ?, ?, ?, ?, ?, ?, ?, ?);`

	now := time.Now()
	lastOnline := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := panel.db.Exec(
		q,
		cmd.Uuid, cmd.TUsername, lastOnline, cmd.ExpireAt, cmd.TrafficAllowed, cmd.PackageDays,
		cmd.Mode, now, 0,
	)
	// ignore if user already exists
	if err != nil {
		err := err.(sqlite3.Error)
		if err.Code != sqlite3.ErrConstraint {
			return err
		}
	}
	return nil
}

func (panel *HiddifyPanel) add2xray(cmd *pb.AddUserCmd) error {
	x := XClient{
		Uuid:    cmd.Uuid,
		Email:   cmd.Uuid + "@zood_server:" + panel.name,
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
	return names, nil
}
