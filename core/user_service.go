package core

import (
	"crypto/md5"
	"fmt"
	"github.com/amin1024/xtelbot/conf"
	"github.com/amin1024/xtelbot/core/repo"
	"github.com/amin1024/xtelbot/core/repo/models"
	"github.com/amin1024/xtelbot/pb"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"time"
)

func NewUserService() *UserService {
	repo.SetupDb("db.db")
	repo.AutoMigrate()
	repo.SetupPackage()
	s := newNodesService()
	userService := &UserService{
		log:          conf.NewLogger(),
		nodesService: s,
	}

	return userService
}

// -----------------------------------------------------------------

type UserService struct {
	log          *zap.SugaredLogger
	nodesService *nodesService
}

// Register a user on local repo and send a "AddClient" request to panels
func (u *UserService) Register(tid uint64, username string, packageName string) error {
	p, err := repo.GetPackage(packageName)
	if err != nil {
		return fmt.Errorf("cannot build the user: %w", err)
	}
	now := time.Now()
	uid := uuid.New().String()
	token := fmt.Sprintf("%x", md5.Sum([]byte(uid)))
	user := &models.Tuser{
		Tid:               tid,
		Username:          username,
		UUID:              uid,
		Token:             token,
		Active:            false,
		AddedToNodesCount: 0,
		TrafficUsage:      0,
		ExpireAt:          now.Add(time.Duration(p.Duration) * 24 * time.Hour).Format(time.RFC3339),
		PackageID:         p.ID,
	}

	if err := repo.SaveOrUpdateUser(user); err != nil {
		u.log.Errorw("cannot register on db: "+err.Error(), "requested from(tid):", user.Tid)
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

// TrafficUsageRunner runs periodically to fetch the latest traffic-usage from xPanels
func (u *UserService) TrafficUsageRunner() {
	// TODO: Handle traffic reset, monthly, weekly, etc
	t := time.Now()
	users, err := repo.GetAllUsers()
	if err != nil {
		u.log.Errorw("[db] unable to list users", "detail", err)
		// TODO: notify admins
		return
	}
	// NOTE: it might be a good idea to run this as a batch goroutine
	for _, user := range users {
		amount := u.nodesService.GetTrafficUsage(user.UUID)
		if amount <= user.TrafficUsage || amount == 0 {
			continue
		}
		user.TrafficUsage = amount
		if err := repo.UpdateUser(user); err != nil {
			u.log.Errorw("[db] Cannot update traffic usage", "uuid", user.UUID, "detail", err)
		}
	}
	elapsed := time.Since(t).Seconds()
	u.log.Infof("successfully updated traffic-usage for %d users in %f seconds\n", len(users), elapsed)
}

func (u *UserService) SpawnRunners() {
	u.log.Info("userService runners spawned")
	go u.TrafficUsageRunner()

	for {
		select {
		case <-time.After(20 * time.Minute): // update traffic-usage every x minutes
			go u.TrafficUsageRunner()
		}
	}
}
