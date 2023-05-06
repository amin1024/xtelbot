package telbot

import (
	"fmt"
	"github.com/amin1024/xtelbot/core/repo/models"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	tele "gopkg.in/telebot.v3"
)

func (b *BotHandler) HandleBankOrder(c tele.Context) error {
	user := c.Get("userObj").(*models.Tuser)

	packageName, ok := b.orderCache[c.Sender().ID]
	if !ok {
		return b.sendMenu(c, msgPurchaseProcessFailed, b.menu)
	}
	delete(b.orderCache, c.Sender().ID)
	p := b.packages[packageName]
	url, err := b.userService.CreateBankTransaction(user, p)
	if err != nil {
		b.log.Errorw("cannot create new purchase", "detail", err)
		return b.sendMenu(c, msgPurchaseProcessFailed)
	}
	printer := message.NewPrinter(language.English)
	msg := fmt.Sprintf(msgYourOrder, p.Name, printer.Sprintf("%d", p.Price), printer.Sprintf("%d", p.Price*10))

	rm := &tele.ReplyMarkup{}
	rm.Inline(rm.Row(rm.URL("💳 پرداخت", url)))
	return b.sendMenu(c, msg, rm)
}

func (b *BotHandler) HandleCard2CardOrder(c tele.Context) error {
	//cardNum, err := b.userService.GetRandomBankCard()
	//if err != nil {
	//	b.log.Error(err)
	//	return c.Send(msgPurchaseProcessFailed)
	//}
	return b.sendMenu(c, msgCard2CardNotAvailable, b.purchaseMethodMenu)
}
