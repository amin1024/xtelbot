package telbot

import (
	"errors"
	"fmt"
	"github.com/amin1024/xtelbot/conf"
	"github.com/amin1024/xtelbot/core"
	"github.com/amin1024/xtelbot/core/e"
	"github.com/amin1024/xtelbot/core/repo"
	"github.com/amin1024/xtelbot/core/repo/models"
	"go.uber.org/zap"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	tele "gopkg.in/telebot.v3"
	"net/url"
	"os"
	"strconv"
	"time"
)

const packagePrefix = "package-"
const packageImgCloudId = "package_img_cloud_id"

// -----------------------------------------------------------------

func NewBotHandler(domainAddr string) *BotHandler {
	log := conf.NewLogger()
	log.Info("Creating new bot")
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN env variable not found")
	}

	pref := tele.Settings{
		Token:     token,
		Poller:    &tele.LongPoller{Timeout: 10 * time.Second},
		ParseMode: tele.ModeMarkdown,
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	userService := core.NewUserService()
	notifyMe := make(chan core.Notification)
	userService.SubscribeNotification(notifyMe)

	h := BotHandler{
		bot:         bot,
		userService: userService,
		domainAddr:  domainAddr,
		log:         log,

		userNotifyChannel: notifyMe,
	}
	// The bot needs at least 1 bank_card to handle purchases
	if cards, err := userService.GetRandomBankCard(); err != nil || len(cards) == 0 {
		log.Fatal("must setup a bank_card")
	}

	records, err := repo.GetKeyVal(packageImgCloudId)
	if err == nil && len(records) >= 1 {
		r := records[0]
		h.packageImageFile = tele.File{
			FileID: r.Value,
		}
	}

	// A middleware to check if the user already registered
	validateUserMiddleware := func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			tid := uint64(c.Sender().ID)
			user, err := userService.Status(tid)
			if errors.Is(err, e.UserNotFound) {
				return c.Send(msgNotRegisteredYet)
			}
			if errors.Is(err, e.BaseError) {
				return c.Send(msgWtf)
			}
			c.Set("userObj", user)
			return next(c)
		}
	}
	activeUserMiddleware := func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			user := c.Get("userObj").(*models.Tuser)
			if !user.Active {
				return c.Send(msgUserNotActive)
			}
			return next(c)
		}
	}

	var purchaseMenu = &tele.ReplyMarkup{}
	// Create a map of "btn unique name" -> "package model"
	availablePackages := make(map[string]*models.Package)
	var btns []tele.Btn
	packages, _ := userService.ListAvailablePackages()
	for _, p := range packages {
		if p.Price == 0 { // We don't want to show free packages to users
			continue
		}
		uniqueName := packagePrefix + strconv.FormatInt(p.ID, 10)
		availablePackages[uniqueName] = p
		btns = append(btns, purchaseMenu.Data(p.Name, uniqueName))
	}
	rows := make([]tele.Row, 2+len(btns)/3)
	for i, btn := range btns {
		i /= 3
		rows[i] = append(rows[i], btn)
	}
	purchaseMenu.Inline(rows...)
	h.packages = availablePackages
	h.purchaseMenu = purchaseMenu

	// define a menu to handle purchase confirmation coming from admins
	var purchaseConfirmMenu = &tele.ReplyMarkup{}
	approve := purchaseConfirmMenu.Data("Approve", "approve-purchase")
	reject := purchaseConfirmMenu.Data("Reject", "reject-purchase")
	purchaseConfirmMenu.Inline(purchaseConfirmMenu.Row(approve, reject))
	h.purchaseConfirmMenu = purchaseConfirmMenu

	bot.Handle("/start", h.Register)
	bot.Handle("/usage", h.TrafficUsage, validateUserMiddleware)
	bot.Handle("/sub", h.Sub, validateUserMiddleware, activeUserMiddleware)
	bot.Handle("/purchase", h.Purchase, validateUserMiddleware)
	bot.Handle(tele.OnPhoto, h.PurchaseReceipt, validateUserMiddleware, activeUserMiddleware)
	bot.Handle(&approve, h.ApprovePurchase)
	bot.Handle(&reject, h.RejectPurchase)

	// Dynamic handler for purchase buttons
	for _, btn := range btns {
		bot.Handle(&btn, h.HandlePackagePurchase, validateUserMiddleware)
	}

	// Setup channel notification
	h.purchaseNotifyChannel = make(chan repo.PurchaseNotify)
	h.adminGroupId = tele.ChatID(-1001514626412) // I was told by god themselves to hard code it xD
	go h.StartNotifyingAdmins()
	go h.StartNotifyingUsers()

	// Start periodic runners
	go h.userService.SpawnRunners()
	return &h
}

// -----------------------------------------------------------------

type BotHandler struct {
	bot         *tele.Bot
	userService *core.UserService
	domainAddr  string

	log *zap.SugaredLogger

	purchaseMenu        *tele.ReplyMarkup
	purchaseConfirmMenu *tele.ReplyMarkup

	packages         map[string]*models.Package
	packageImageFile tele.File

	adminGroupId          tele.ChatID
	purchaseNotifyChannel chan repo.PurchaseNotify

	userNotifyChannel chan core.Notification
}

func (b *BotHandler) Start() {
	b.log.Info("Starting the telegram-bot")
	b.bot.Start()
}

// StartNotifyingAdmins loops over a channel and sends a photo with caption to the admin channel
func (b *BotHandler) StartNotifyingAdmins() {
	printer := message.NewPrinter(language.English)
	for n := range b.purchaseNotifyChannel {
		// add purchase item id to buttons
		tmpPurchaseMenu := b.purchaseConfirmMenu
		buttons := tmpPurchaseMenu.InlineKeyboard[0]
		var newButtons []tele.Btn
		for _, btn := range buttons {
			newButtons = append(newButtons, tmpPurchaseMenu.Data(btn.Text, btn.Unique, strconv.FormatInt(n.Purchase.ID, 10)))
		}
		tmpPurchaseMenu.Inline(newButtons)

		msg := fmt.Sprintf(msgPurchaseNotification, n.Tuser.Username, n.Tuser.Tid, printer.Sprintf("%d", n.Purchase.Price), n.Purchase.PackageName)
		n.Photo.Caption = msg
		if _, err := b.bot.Send(b.adminGroupId, n.Photo, tmpPurchaseMenu); err != nil {
			b.log.Errorw("[fatal] cannot notify admins on new purchase event", "detail", err)
		}
	}
}

func (b *BotHandler) StartNotifyingUsers() {
	for notify := range b.userNotifyChannel {
		var msg string
		switch notify.Type {
		case core.PurchaseSuccessful:
			packageName := notify.Extra.(*models.Purchase).PackageName
			msg = fmt.Sprintf(msgPurchaseSuccess, packageName)

		case core.PurchaseRejected:
			msg = msgPurchaseRejected

		case core.UserMaxTrafficReached:
			msg = msgUserTrafficLimitReached
		}

		_, err := b.bot.Send(&tele.User{ID: int64(notify.User.Tid)}, msg)
		if err != nil {
			b.log.Error(err)
		}
	}
}

func (b *BotHandler) Register(c tele.Context) error {
	b.log.Info("Received: /start")
	tid := uint64(c.Sender().ID)
	username := c.Sender().Username
	// Check if user is already registered
	_, err := b.userService.Status(tid)
	if err == nil {
		//return c.Send(msgAlreadyRegistered)
		return c.Send(msgRegistrationSuccess)
	}
	if !errors.Is(err, e.UserNotFound) { // Any error other than UserNotFound considered as 5xx
		return c.Send(msgWtf)
	}

	// Register the user on bot and every available panel

	err = b.userService.Register(tid, username, "")
	if err != nil {
		return c.Send(msgRegistrationFailed)
	}
	return c.Send(msgRegistrationSuccess)
}

func (b *BotHandler) TrafficUsage(c tele.Context) error {
	user := c.Get("userObj").(*models.Tuser)
	remaining := user.R.Package.TrafficAllowed - user.TrafficUsage
	return c.Send(fmt.Sprintf(msgTraffic, user.TrafficUsage, remaining))
}

func (b *BotHandler) Sub(c tele.Context) error {
	user := c.Get("userObj").(*models.Tuser)
	baseUrl, _ := url.JoinPath("https://", b.domainAddr, "/v1/sub/")
	return c.Send(msgSubLinkAndroid + baseUrl + user.Token)
}

func (b *BotHandler) Purchase(c tele.Context) error {
	//printer := message.NewPrinter(language.English)
	//var msg string
	//for _, pck := range b.packages {
	//	msg = msg + "\n\n" + printer.Sprintf(pck.Description, pck.Name, pck.Price)
	//}

	//return c.Send(msgPackageDetailsHeader+msg+"\n.", b.purchaseMenu)
	if !b.packageImageFile.InCloud() {
		b.packageImageFile = tele.FromDisk("misc/packages_1.jpg")
	}
	msg := tele.Photo{
		File:    b.packageImageFile,
		Width:   0,
		Height:  0,
		Caption: msgPackageDetailWithPicture,
	}
	err := c.Send(&msg, b.purchaseMenu)
	if err == nil && !b.packageImageFile.InCloud() {
		_ = repo.SetKeyVal(packageImgCloudId, msg.File.FileID)
		b.packageImageFile.FileID = msg.File.FileID
	}
	return err
}

func (b *BotHandler) HandlePackagePurchase(c tele.Context) error {
	buttonUniqueName := c.Callback().Unique
	p, ok := b.packages[buttonUniqueName]
	if !ok {
		b.log.Errorw("invalid callback button", "uniqueName", buttonUniqueName)
		return c.Send(msgInvalidButtonCallback)
	}
	user := c.Get("userObj").(*models.Tuser)
	err := b.userService.CreatePurchase(user, p, c.Message().ID)
	if err != nil {
		b.log.Errorw("cannot create new purchase", "detail", err)
		return c.Send(msgPurchaseProcessFailed)
	}

	cardNum, err := b.userService.GetRandomBankCard()
	if err != nil {
		b.log.Error(err)
		return c.Send(msgPurchaseProcessFailed)
	}
	printer := message.NewPrinter(language.English)
	_ = c.Delete()
	return c.Send(fmt.Sprintf(msgPurchasePackage, p.Name, printer.Sprintf("%d", p.Price), printer.Sprintf("%d", p.Price*10), cardNum))
}

func (b *BotHandler) PurchaseReceipt(c tele.Context) error {
	user := c.Get("userObj").(*models.Tuser)
	pn, err := b.userService.ProcessPurchaseOnReceipt(user)
	if errors.Is(err, e.ReceiptPhotoWithoutActualPurchase) {
		return c.Send(msgPhotoWithoutPurchase)
	}
	if errors.Is(err, e.BaseError) {
		return c.Send(msgPurchaseProcessFailed)
	}
	// Notify admins about this new purchase
	pn.Photo = c.Message().Photo
	b.purchaseNotifyChannel <- pn
	return c.Send(msgPurchaseWaitPlease)
}

func (b *BotHandler) ApprovePurchase(c tele.Context) error {
	purchaseId := c.Callback().Data
	if purchaseId == "" {
		b.log.Errorw("[unreachable error reached] purchaseId is empty")
	}
	if err := b.userService.ConfirmPurchase(purchaseId); err != nil {
		if errors.Is(err, e.PurchaseAlreadyProcessed) {
			b.bot.Reply(c.Callback().Message, msgPurchaseAlreadyProcessed)
			return err
		}
		b.log.Error(err)
		// Notify the admins about the inconvenience
		b.bot.Reply(c.Callback().Message, msgPurchaseConfirmationFailed)
		return err
	}
	b.bot.Reply(c.Callback().Message, msgPurchaseConfirmationSuccess)
	return nil
}

func (b *BotHandler) RejectPurchase(c tele.Context) error {
	purchaseId := c.Callback().Data
	if purchaseId == "" {
		b.log.Fatal("[unreachable error reached] purchaseId is empty")
	}
	if err := b.userService.RejectPurchase(purchaseId); err != nil {
		if errors.Is(err, e.PurchaseAlreadyProcessed) {
			b.bot.Reply(c.Callback().Message, msgPurchaseAlreadyProcessed)
			return err
		}
		b.log.Error(err)
		// Notify the admins about the inconvenience
		b.bot.Reply(c.Callback().Message, msgPurchaseConfirmationFailed)
		return err
	}
	b.bot.Reply(c.Callback().Message, msgPurchaseConfirmationSuccess)
	return nil
}
