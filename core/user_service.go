package core

import (
	"fmt"
	"github.com/amin1024/xtelbot/core/repo"
	"github.com/amin1024/xtelbot/core/repo/models"
	"github.com/amin1024/xtelbot/pb"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"time"
)

var log *zap.SugaredLogger

func init() {
	l, _ := zap.NewProduction()
	log = l.Sugar()
}

func NewUserService() *UserService {
	repo.SetupDb()
	repo.SetupPackage()
	s := newNodesService()
	userService := &UserService{
		nodesService: s,
	}

	return userService
}

// -----------------------------------------------------------------

type UserService struct {
	nodesService *nodesService
}

// Register a user on local repo and send a "AddClient" request to panels
func (u *UserService) Register(tid uint64, username string, packageName string) error {
	p, err := repo.GetPackage(packageName)
	if err != nil {
		return fmt.Errorf("cannot build the user: %w", err)
	}
	now := time.Now()
	user := &models.Tuser{
		Tid:               tid,
		Username:          username,
		UUID:              uuid.New().String(),
		Active:            false,
		AddedToNodesCount: 0,
		TrafficUsage:      0,
		ExpireAt:          now.Add(time.Duration(p.Duration) * 24 * time.Hour).String(),
		PackageID:         p.ID,
	}

	if err := repo.SaveOrUpdateUser(user); err != nil {
		log.Errorw("cannot register on db: "+err.Error(), "requested from(tid):", user.Tid)
		return err
	}
	cmd := pb.AddUserCmd{
		Tid:            user.Tid,
		TUsername:      user.Username,
		Uuid:           user.UUID,
		TrafficAllowed: p.TrafficAllowed,
		ExpireAt:       user.ExpireAt,
		PackageDays:    p.Duration,
		Mode:           p.ResetMode,
	}
	// Add this telegram user to any alive xNode (aka panels)
	n, err := u.nodesService.AddUser(&cmd)
	if err != nil {
		return err
	}
	user.AddedToNodesCount = int64(n)
	user.Active = true
	if err := repo.UpdateUser(user); err != nil {
		return err
	}
	return nil
}

func (u *UserService) Unregister(uid uint64) error {
	return nil
}

func (u *UserService) Upgrade(uint642 uint64) error {
	return nil
}

func (u *UserService) Status(uid uint64) (*models.Tuser, error) {
	user, err := repo.GetUser(uid)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// DisabledCallback is a callback-function to be called by XrayPanels when
// they have disabled a user due to traffic limit
func (u *UserService) DisabledCallback() error {
	return nil
}

// -------------------------------------------------------------------
//
//func (u *UserService) buildUser(uid uint64, username string) (*models.Tuser, error) {
//	p, err := repo.GetDefaultPackage()
//	if err != nil {
//		return nil, fmt.Errorf("cannot build the user: %w", err)
//	}
//	user := &models.Tuser{
//		Tid:               uid,
//		Username:          username,
//		Active:            false,
//		AddedToNodesCount: 0,
//		TrafficUsage:      0,
//		ExpireAt:          "",
//		PackageID:         p,
//	}
//	user.SetPackage()
//	return user, nil
//}
