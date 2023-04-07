package core

import (
	"context"
	"fmt"
	"github.com/amin1024/xtelbot/conf"
	"github.com/amin1024/xtelbot/core/repo"
	"github.com/amin1024/xtelbot/core/repo/models"
	"github.com/amin1024/xtelbot/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sync"
)

type xNode struct {
	data   *models.Xnode
	client pb.XNodeGrpcClient
}

// -----------------------------------------------------------------

type nodesService struct {
	nodes []*xNode
	log   *zap.SugaredLogger
}

func newNodesService() *nodesService {
	log := conf.NewLogger()
	nodesModels, err := repo.GetXNodes()
	if err != nil {
		log.Fatal(err)
	}
	var nodes []*xNode
	for _, node := range nodesModels {
		client, err := ConnectToXNode(node)
		if err != nil {
			log.Errorw("XNode warmup failed", "~address", node.Address)
			continue
		}
		nodes = append(nodes, &xNode{
			data:   node,
			client: client,
		})
	}
	log.Infof("connected to %d XNodes", len(nodes))
	// TODO: fatal error when there is no available client connection (success==0)
	ns := nodesService{
		nodes: nodes,
		log:   log,
	}
	return &ns
}

func (x *nodesService) ListXNodes() ([]xNode, error) {
	return []xNode{}, nil
}

// AddUser sends a AddUserCmd to every server, returns the number of successful adds.
func (x *nodesService) AddUser(cmd *pb.AddUserCmd) (int, error) {
	success := 0
	wg := sync.WaitGroup{}
	for _, node := range x.nodes {
		wg.Add(1)
		go func(node *xNode) {
			defer wg.Done()
			_, err := node.client.AddUser(context.Background(), cmd)
			if err != nil {
				x.log.Errorw("panel cannot add user:"+err.Error(), "~failed_node", node.data.Address)
				// TODO: notify the administrator
				return
			}
			success++
		}(node)
	}

	wg.Wait()

	x.log.Infof("Added %d users on %d servers\n", success, len(x.nodes))
	if success == 0 {
		return 0, fmt.Errorf("failed to register user on any of the xNodes: %d - %s", cmd.Tid, cmd.TUsername)
	}
	return success, nil
}

//------------------------------------------------

func ConnectToXNode(node *models.Xnode) (pb.XNodeGrpcClient, error) {
	c, err := grpc.DialContext(
		context.Background(),
		node.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	client := pb.NewXNodeGrpcClient(c)
	_, err = client.Ping(context.Background(), &pb.Empty{})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func AddXNode(node *models.Xnode) error {
	repo.AutoMigrate()
	// TODO: we need a way to populate the new server by all the users we already have
	// Find a better way to support data consistency! all nodes must be on the same state
	_, err := ConnectToXNode(node)
	if err != nil {
		return err
	}
	if err := repo.SaveOrUpdateXNode(node); err != nil {
		return err
	}
	return nil
}
