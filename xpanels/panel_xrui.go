package xpanels

import (
	"context"
	"github.com/amin1024/xtelbot/pb"
)

func NewXuiPanel(xs *XrayService, conf map[string]string) *XuiPanel {
	return &XuiPanel{}
}

type XuiPanel struct {
	pb.UnimplementedXNodeGrpcServer
}

func (panel *XuiPanel) AddUser(ctx context.Context, cmd *pb.AddUserCmd) (*pb.Response, error) {
	return &pb.Response{}, nil
}
