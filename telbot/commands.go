package telbot

import (
	"errors"
	"fmt"
	"github.com/amin1024/xtelbot/core"
	"github.com/amin1024/xtelbot/core/e"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
	"os"
	"time"
)

var log *zap.SugaredLogger

func init() {
	l, _ := zap.NewProduction()
	log = l.Sugar()
}

// -----------------------------------------------------------------

func UserIdMiddleware(next tele.HandlerFunc) tele.HandlerFunc {
	return func(ctx tele.Context) error {
		u := ctx.Sender()
		if u != nil {
			ctx.Set("tid", uint64(u.ID))
		} else {
			ctx.Set("tid", 0)
		}
		return nil
	}
}

func NewBotHandler() *BotHandler {
	log.Info("Creating new bot")
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN env variable not found")
	}

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return nil
	}
	//bot.Use(UserIdMiddleware)

	h := BotHandler{
		bot:         bot,
		userService: core.NewUserService(),
	}
	bot.Handle("/hello", h.Hi)
	bot.Handle("/start", h.Register)
	bot.Handle("/usage", h.TrafficUsage)
	bot.Handle("/sub", h.Sub)

	bot.Handle("/hi", func(c tele.Context) error {
		return c.Send("Hi!")
	})

	return &h
}

// -----------------------------------------------------------------

type BotHandler struct {
	bot         *tele.Bot
	userService *core.UserService
}

func (b *BotHandler) Start() {
	log.Info("Starting the telegram-bot")
	b.bot.Start()
}

func (b *BotHandler) Hi(c tele.Context) error {
	log.Info("Received: /Hello")
	return c.Send("Hi bitch!")
}

func (b *BotHandler) Register(c tele.Context) error {
	log.Info("Received: /start")
	tid := uint64(c.Sender().ID)
	username := c.Sender().Username
	// Check if user is already registered
	_, err := b.userService.Status(tid)
	if err == nil {
		return c.Send(msgAlreadyRegistered)
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
	tid := uint64(c.Sender().ID)
	user, err := b.userService.Status(tid)
	if errors.Is(err, e.BaseError) {
		return c.Send(msgWtf)
	}
	remaining := user.R.Package.TrafficAllowed - user.TrafficUsage
	return c.Send(fmt.Sprintf(msgTraffic, user.TrafficUsage, remaining))
}

func (b *BotHandler) Sub(c tele.Context) error {
	tid := uint64(c.Sender().ID)
	user, err := b.userService.Status(tid)
	if errors.Is(err, e.BaseError) {
		return c.Send(msgWtf)
	}
	return c.Send(fmt.Sprintf(msgTmpSubLink, user.UUID))
}
