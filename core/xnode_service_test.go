package core

import (
	"context"
	"github.com/amin1024/xtelbot/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"testing"
)

// implements pb.XNodeGrpcClient
type mockedClient struct {
	mock.Mock
}

func (m *mockedClient) AddUser(ctx context.Context, in *pb.AddUserCmd, opts ...grpc.CallOption) (*pb.Response, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockedClient) Ping(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.Empty, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockedClient) GetSub(ctx context.Context, in *pb.UserInfoReq, opts ...grpc.CallOption) (*pb.SubContent, error) {
	//TODO implement me
	panic("implement me")
}

func (m *mockedClient) GetUserInfo(ctx context.Context, in *pb.UserInfoReq, opts ...grpc.CallOption) (*pb.UserInfo, error) {
	//args := m.Called(mock.Anything, mock.Anything, mock.Anything)
	return &pb.UserInfo{CurrentUsage: 100}, nil
}

func (m *mockedClient) UpgradeUserPackage(ctx context.Context, in *pb.AddPackageCmd, opts ...grpc.CallOption) (*pb.Response, error) {
	return &pb.Response{}, nil
}

//
//func newMockedNodesService() *NodesService {
//	return
//}

func TestNodesService_GetTrafficUsage(t *testing.T) {
	s := &NodesService{
		nodes: []*xNode{
			&xNode{
				data:   nil,
				client: new(mockedClient),
			},
			&xNode{
				data:   nil,
				client: new(mockedClient),
			},
		},
		log: nil,
	}

	amount := s.GetTrafficUsage("aa")
	assert.Equal(t, amount, float32(200.0))
}
