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

const packageImgCloudId = "package_img_cloud_id"

// -----------------------------------------------------------------

func NewBotHandler(userService *core.UserService, domainAddr string) *BotHandler {
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

	notifyMe := make(chan core.Notification)
	userService.SubscribeNotification(notifyMe)

	h := BotHandler{
		bot:         bot,
		userService: userService,
		domainAddr:  domainAddr,
		log:         log,

		orderCache:        make(map[int64]string),
		userNotifyChannel: notifyMe,
	}
	// The bot needs at least 1 bank_card to handle purchases
	if cards, err := userService.GetRandomBankCard(); err != nil || len(cards) == 0 {
		log.Fatal("must setup a bank_card")
	}

	records, err := repo.GetKeyVal(packageImgCloudId)
	if err == nil && len(records) >= 1 {
		r := records[len(records)-1]
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
				return fmt.Errorf(msgUserNotActive)
			}
			return next(c)
		}
	}

	packages, _ := userService.ListAvailablePackages()
	// Build main menu step by step
	// 1- Build purchase menu
	h.buildPurchaseMenu(packages, validateUserMiddleware)
	h.buildPurchaseMethodMenu(validateUserMiddleware)
	// 2- Build traffic usage menu
	h.buildUsageMenu(backButton)
	// 3- Build sub menu
	h.buildSubMenu(backButton)
	h.buildHelpMenu(backButton)
	// define a menu to handle purchase confirmation coming from admins
	h.buildAdminConfirmationMenu()

	h.buildMainMenu(validateUserMiddleware, activeUserMiddleware)

	bot.Handle("/start", h.Register)
	bot.Handle(tele.OnPhoto, h.PurchaseReceipt, validateUserMiddleware, activeUserMiddleware)
	bot.Handle(&backButton, h.MainMenu)

	// Setup channel notification
	h.purchaseNotifyChannel = make(chan core.AdminPurchaseNotify)
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

	menu                *tele.ReplyMarkup
	purchaseMenu        *tele.ReplyMarkup
	purchaseConfirmMenu *tele.ReplyMarkup
	purchaseMethodMenu  *tele.ReplyMarkup
	usageMenu           *tele.ReplyMarkup
	helpMenu            *tele.ReplyMarkup
	subMenu             *tele.ReplyMarkup

	packages         map[string]*models.Package
	packageImageFile tele.File

	adminGroupId          tele.ChatID
	purchaseNotifyChannel chan core.AdminPurchaseNotify
	// a simple cache to hold which package the user wants to buy
	orderCache map[int64]string

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

		_, err := b.bot.Send(&tele.User{ID: int64(notify.User.Tid)}, msg, b.menu)
		if err != nil {
			b.log.Error(err)
		}
	}
}

func (b *BotHandler) sendMenu(c tele.Context, what interface{}, opts ...interface{}) error {
	_, err := b.bot.Edit(c.Message(), what, opts...)
	return err
}

func (b *BotHandler) send(c tele.Context, what interface{}, opts ...interface{}) error {
	return c.Send(what, opts...)
}

func (b *BotHandler) MainMenu(c tele.Context) error {
	return b.sendMenu(c, msgMenu, b.menu)
}

func (b *BotHandler) Help(c tele.Context) error {
	return b.sendMenu(c, "Help", b.helpMenu)
}

func (b *BotHandler) Register(c tele.Context) error {
	b.log.Info("Received: /start")
	tid := uint64(c.Sender().ID)
	username := c.Sender().Username
	// Check if user is already registered
	_, err := b.userService.Status(tid)
	if err == nil {
		//return c.Send(msgAlreadyRegistered)
		_ = c.Send(msgRegistrationSuccess)
		return b.send(c, msgMenu, b.menu)
	}
	if !errors.Is(err, e.UserNotFound) { // Any error other than UserNotFound considered as 5xx
		return b.send(c, msgMenu, b.menu)
	}

	// Register the user on bot and every available panel

	err = b.userService.Register(tid, username, "")
	if err != nil {
		return c.Send(msgRegistrationFailed, b.menu)
	}
	_ = c.Send(msgRegistrationSuccess)
	return b.send(c, msgMenu, b.menu)
}

func (b *BotHandler) TrafficUsage(c tele.Context) error {
	user := c.Get("userObj").(*models.Tuser)
	remaining := user.R.Package.TrafficAllowed - user.TrafficUsage
	if remaining < 0 {
		remaining = 0
	}
	return b.sendMenu(c, fmt.Sprintf(msgTraffic, user.TrafficUsage, remaining), b.usageMenu)
}

func (b *BotHandler) Sub(c tele.Context) error {
	user := c.Get("userObj").(*models.Tuser)
	baseUrl, _ := url.JoinPath("https://", b.domainAddr, "/v1/sub/")
	return b.sendMenu(c, msgSubLinkAndroid+baseUrl+user.Token, b.subMenu)
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
	_ = c.Delete()
	_ = c.Send(&msg)
	err := b.send(c, msgPackageChoosePackage, b.purchaseMenu)
	if err == nil && !b.packageImageFile.InCloud() {
		_ = repo.SetKeyVal(packageImgCloudId, msg.File.FileID)
		b.packageImageFile.FileID = msg.File.FileID
	}
	return err
}

func (b *BotHandler) ChoosePurchaseMethod(c tele.Context) error {
	buttonUniqueName := c.Callback().Unique
	p, ok := b.packages[buttonUniqueName]
	if !ok {
		b.log.Errorw("invalid callback button", "uniqueName", buttonUniqueName)
		return b.sendMenu(c, msgMenu, b.menu)
	}
	b.orderCache[c.Sender().ID] = buttonUniqueName
	printer := message.NewPrinter(language.English)
	msg := fmt.Sprintf(msgPurchaseChooseMethod, p.Name, printer.Sprintf("%d", p.Price), printer.Sprintf("%d", p.Price*10))
	return b.sendMenu(c, msg, b.purchaseMethodMenu)
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
