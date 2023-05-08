package core

import (
	"crypto/md5"
	"fmt"
	"github.com/amin1024/xtelbot/conf"
	"github.com/amin1024/xtelbot/core/e"
	"github.com/amin1024/xtelbot/core/repo"
	"github.com/amin1024/xtelbot/core/repo/models"
	"github.com/amin1024/xtelbot/pb"
	"github.com/friendsofgo/errors"
	"github.com/google/uuid"
	"github.com/volatiletech/null/v8"
	"go.uber.org/zap"
	"math/rand"
	"strconv"
	"time"
)

func NewUserService(terminal PaymentTerminal) *UserService {
	log := conf.NewLogger()
	repo.SetupDb("db.db")
	repo.AutoMigrate()
	repo.SetupPackage()
	s := NewNodesService()
	userService := &UserService{
		log:             log,
		nodesService:    s,
		paymentTerminal: terminal,
	}

	return userService
}

// -----------------------------------------------------------------

type UserService struct {
	notificationChannels []chan<- Notification

	log          *zap.SugaredLogger
	nodesService *NodesService

	paymentTerminal PaymentTerminal
}

func (u *UserService) SubscribeNotification(ch chan<- Notification) {
	u.notificationChannels = append(u.notificationChannels, ch)
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
		Tid:       user.Tid,
		TUsername: user.Username,
		Uuid:      user.UUID,
		Package: &pb.Package{
			TrafficAllowed: p.TrafficAllowed,
			ExpireAt:       user.ExpireAt,
			PackageDays:    p.Duration,
			Mode:           p.ResetMode,
		},
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

func (u *UserService) ListAvailablePackages() ([]*models.Package, error) {
	packages, err := repo.GetAllPackages()
	if err != nil {
		u.log.Fatalw("[db] unable to list packages", "detail", err)
	}
	return packages, err
}

// Upgrade the user to a new package then send a command to all the available servers
func (u *UserService) Upgrade(user *models.Tuser, pck *models.Package) error {
	user.PackageID = pck.ID
	if err := repo.UpdateUser(user); err != nil {
		return err
	}
	if err := u.nodesService.UpgradeUserPackage(user.UUID, pck); err != nil {
		// TODO: We want to keep track of failed servers so that we retry again later
		return e.PackageUpgradeFailedByXNodes
	}
	u.log.Infow("Successfully upgraded user", "userTid", user.Tid, "username", user.Username, "packageId", pck.ID)
	return nil
}

func (u *UserService) Status(uid uint64) (*models.Tuser, error) {
	user, err := repo.GetUserByTid(uid)
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

//func (u *UserService) CreatePurchase(user *models.Tuser, pck *models.Package, msgId int) error {
//	p := &models.Purchase{
//		TuserID:     user.ID,
//		PackageID:   pck.ID,
//		Price:       pck.Price,
//		PackageName: pck.Name,
//		Status:      int64(repo.PurchaseUnknown),
//	}
//	if err := repo.InsertPurchase(p); err != nil {
//		return fmt.Errorf("[db]: %w", e.BaseError)
//	}
//	return nil
//}

// CreateBankTransaction communicate with bank-terminal to get a new transaction and returns transaction id
func (u *UserService) CreateBankTransaction(user *models.Tuser, pck *models.Package) (string, error) {
	p := &models.Purchase{
		TuserID:     user.ID,
		PackageID:   pck.ID,
		Price:       pck.Price,
		PackageName: pck.Name,
		Status:      int64(repo.PurchaseWaitingForBankCallback),
	}
	// Create a new purchase/order and also disable previous ones
	if err := repo.CreatePurchase(p); err != nil {
		return "", fmt.Errorf("[db]: %w", e.BaseError)
	}
	transId, err := u.paymentTerminal.CreateToken(p.Price, p.ID)
	if err != nil {
		// notify admins: bank problem
		return "", err
	}
	// update order, add transaction id
	p.TransactionID = null.StringFrom(transId)
	if err := repo.UpdatePurchase(p); err != nil {
		return "", err
	}
	return u.paymentTerminal.CreateRedirectUrl(transId), nil
}

func (u *UserService) VerifyBankTransaction(parameters CallbackParameters) (*models.Purchase, error) {
	p, err := repo.GetPurchaseById(parameters.OrderId)
	if parameters.Amount <= 0 || err != nil {
		return p, e.PurchaseNotFound
	}
	vr, err := u.paymentTerminal.VerifyOrder(parameters)
	if err != nil {
		u.log.Warnw("order verification failed", "code", vr.Code, "order_id", p.ID, "user", p.R.Tuser.Username, "detail", err)
		return p, err
	}
	if vr.Code != 0 {
		return p, e.OrderNotVerified
	}

	remoteOrderId, _ := strconv.ParseInt(vr.OrderId, 10, 64)
	if p.ID != remoteOrderId || p.Price != vr.Amount {
		u.log.Errorw("[unreachable code] invalid purchase received from bank!", "db:purchaseId", p.ID, "remote purchase", vr)
	}
	// confirm on db
	p.ShaparakRef = null.StringFrom(vr.ShaparakRef)
	p.Status = int64(repo.PurchaseConfirmed)
	if err := repo.UpdatePurchase(p); err != nil {
		u.log.Errorw("[db] cannot update purchase table", "detail", err)
		return p, e.FatalErr
	}

	// upgrade user on every xPanel
	if err := u.Upgrade(p.R.Tuser, p.R.Package); err != nil {
		u.log.Errorw("[silent] failed to upgrade user package", "detail", err)
	}

	// Notify user on successful purchase
	n := Notification{
		Type:  PurchaseSuccessful,
		User:  p.R.Tuser,
		Extra: p,
	}
	u.notifyUser(n)
	return p, nil
}

// ProcessPurchaseOnReceipt process a purchase when the user paid the price and sent a receipt to the admins
func (u *UserService) ProcessPurchaseOnReceipt(user *models.Tuser) (AdminPurchaseNotify, error) {
	var pn AdminPurchaseNotify
	purchase, err := repo.LastPurchasesByUserId(user.ID, repo.PurchaseUnknown)
	if errors.Is(err, e.PurchaseNotFound) {
		return pn, e.ReceiptPhotoWithoutActualPurchase
	}
	if err != nil {
		u.log.Errorw("[db] failed to get latest purchase", "userId", user.ID, "detail", err)
		return pn, errors.Wrap(err, e.BaseError.Error())
	}

	pn.Purchase = purchase
	pn.Tuser = user

	// Disable older purchases and set the current purchase as IsProcessing.
	// The goal is to be assured that every user has only 1 active purchase on his/her basket
	if err := repo.SetPurchaseAsProcessing(purchase); err != nil {
		return pn, err
	}
	return pn, nil
}

func (u *UserService) ConfirmPurchase(purchaseId string) error {
	return u.processPurchase(purchaseId, repo.PurchaseConfirmed)
}

func (u *UserService) RejectPurchase(purchaseId string) error {
	return u.processPurchase(purchaseId, repo.PurchaseRejected)
}

// processPurchase process a purchase when admin acts upon it
func (u *UserService) processPurchase(purchaseId string, newStatus repo.PurchaseStatus) error {
	pid, err := strconv.ParseInt(purchaseId, 10, 64)
	if err != nil {
		return e.InvalidPurchaseIdFormat
	}
	purchase, err := repo.GetPurchaseById(pid)
	if purchase.Status != int64(repo.PurchaseUnknown) {
		return e.PurchaseAlreadyProcessed
	}
	if err != nil {
		return e.PurchaseNotFound
	}

	if newStatus == repo.PurchaseConfirmed {
		// admin confirmed the purchase, lets upgrade the user package
		if err := u.Upgrade(purchase.R.Tuser, purchase.R.Package); err != nil {
			return err
		}
	}

	purchase.ProcessedAt = null.TimeFrom(time.Now())
	purchase.Status = int64(newStatus)
	if err := repo.UpdatePurchase(purchase); err != nil {
		u.log.Errorw("cannot update purchase's status", "detail", err)
		return err
	}
	// Publish a new notification: users must be notified of the process result
	n := Notification{
		User:  purchase.R.Tuser,
		Extra: purchase,
	}
	if newStatus == repo.PurchaseConfirmed {
		n.Type = PurchaseSuccessful
	} else if newStatus == repo.PurchaseRejected {
		n.Type = PurchaseRejected
	}
	u.notifyUser(n)
	return nil
}

func (u *UserService) GetRandomBankCard() (string, error) {
	// Warning: users will be able to extract all the available bank cards!
	values, err := repo.GetKeyVal("bank_card")
	if err != nil || len(values) == 0 {
		return "", e.BankCardNotFound
	}
	rand.Seed(time.Now().Unix())
	return values[rand.Intn(len(values))].Value, nil
}

func (u *UserService) notifyUser(n Notification) {
	go func(n Notification) {
		for _, ch := range u.notificationChannels {
			ch <- n
		}
	}(n)
}
