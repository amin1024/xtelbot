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
	"time"
)

type xNode struct {
	data   *models.Xnode
	client pb.XNodeGrpcClient
}

// -----------------------------------------------------------------

type NodesService struct {
	nodes []*xNode
	log   *zap.SugaredLogger

	sync.Mutex
}

var nodesServiceSingleton *NodesService

func NewNodesService() *NodesService {
	if nodesServiceSingleton != nil {
		return nodesServiceSingleton
	}
	log := conf.NewLogger()
	nodesModels, err := repo.GetXNodes()
	if err != nil {
		log.Fatal(err)
	}

	srv := NodesService{}
	var nodes []*xNode
	for _, node := range nodesModels {
		client, err := srv.connectToXNode(node)
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

	srv.nodes = nodes
	srv.log = log

	return &srv
}

func (x *NodesService) ListXNodes() ([]xNode, error) {
	return []xNode{}, nil
}

// AddUser sends a AddUserCmd to every server, returns the number of successful adds.
func (x *NodesService) AddUser(cmd *pb.AddUserCmd) (int, error) {
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

func (x *NodesService) GetSubs(user *models.Tuser) []string {
	var result []string
	wg := sync.WaitGroup{}
	mu := sync.Mutex{}

	uInfo := &pb.UserInfoReq{
		Tid:       user.Tid,
		TUsername: user.Username,
		Uuid:      user.UUID,
	}
	for _, node := range x.nodes {
		wg.Add(1)
		go func(node *xNode) {
			defer wg.Done()
			sc, err := node.client.GetSub(context.Background(), uInfo)
			if err != nil {
				x.log.Errorw("unable to receive sub-content", "node", node.data.Address, "remote error", err)
				return
			}
			mu.Lock()
			result = append(result, sc.GetContent())
			mu.Unlock()
		}(node)
	}
	wg.Wait()
	return result
}

func (x *NodesService) GetTrafficUsage(uid string) float32 {
	x.Lock()
	defer x.Unlock()
	ch := make(chan float32, len(x.nodes))
	for _, node := range x.nodes {
		go func(node *xNode) {
			r, err := node.client.GetUserInfo(context.Background(), &pb.UserInfoReq{Uuid: uid})
			if err != nil {
				x.log.Errorw("[xnode] GetTrafficUsage error", "uuid", uid, "detail", err)
				ch <- 0
				return
			}
			ch <- r.GetCurrentUsage()
		}(node)
	}
	var totalUsage float32
	for i := 0; i < len(x.nodes); i++ {
		totalUsage = totalUsage + <-ch
	}
	return totalUsage
}

func (x *NodesService) UpgradeUserPackage(userUuid string, pck *models.Package) error {
	expireAt := time.Now().Add(time.Hour * 24 * time.Duration(pck.Duration))
	cmd := &pb.AddPackageCmd{
		Uuid: userUuid,
		Package: &pb.Package{
			TrafficAllowed: pck.TrafficAllowed,
			ExpireAt:       expireAt.Format(time.RFC3339),
			PackageDays:    pck.Duration,
			Mode:           pck.ResetMode,
		},
	}
	wg := sync.WaitGroup{}
	var fails error
	for _, node := range x.nodes {
		wg.Add(1)
		go func(node *xNode) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := node.client.UpgradeUserPackage(ctx, cmd)
			if err != nil {
				x.log.Errorw("xNode failed to upgrade user: ", "xNode", node.data.Address, "uuid", userUuid, "packageId", pck.ID, "detail", err)
				fails = fmt.Errorf("failed xnode: %s", node.data.Address)
			}
		}(node)
	}
	wg.Wait()
	return fails
}

//------------------------------------------------

func (x *NodesService) connectToXNode(node *models.Xnode) (pb.XNodeGrpcClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, err := grpc.DialContext(
		ctx,
		node.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
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

func (x *NodesService) AddXNode(node *models.Xnode) error {
	// Find a better way to support data consistency! all nodes must be on the same state
	client, err := x.connectToXNode(node)
	if err != nil {
		return err
	}
	if err := repo.SaveOrUpdateXNode(node); err != nil {
		return err
	}
	// TODO: improve it by sending batch users
	// Sync the users one by one
	users, err := repo.GetAllUsersWithPackages()
	if err != nil {
		return err
	}
	var errs []error
	for _, user := range users {
		pck := user.R.Package
		expireAt := time.Now().Add(time.Hour * 24 * time.Duration(pck.Duration))
		cmd := &pb.AddUserCmd{
			Tid:       user.Tid,
			TUsername: user.Username,
			Uuid:      user.UUID,
			Package: &pb.Package{
				TrafficAllowed: pck.TrafficAllowed,
				ExpireAt:       expireAt.Format(time.RFC3339),
				PackageDays:    pck.Duration,
				Mode:           pck.ResetMode,
			},
		}
		if _, err := client.AddUser(context.Background(), cmd); err != nil {
			errs = append(errs, fmt.Errorf("tid: %d - %w", user.Tid, err))
		}
	}
	if len(errs) != 0 {
		x.log.Warnw("failed to sync users", "detail", errs)
	}
	x.log.Infof("successfully synced %d users", len(users))
	return nil
}
