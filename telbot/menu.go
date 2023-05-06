package telbot

import (
	"github.com/amin1024/xtelbot/core/repo/models"
	tele "gopkg.in/telebot.v3"
	"strconv"
)

const (
	btnTrafficUsage   = "usage"
	btnTrafficUsageFa = "گزارش مصرف"

	btnPurchaseSub   = "purchase-sub"
	btnPurchaseSubFa = "خرید اشتراک"

	btnPurchaseCard2Card   = "purchase-method-card"
	btnPurchaseCard2CardFa = "کارت به کارت"
	btnPurchaseNextpay     = "purchase-method-nextpay"
	btnPurchaseNextpayFa   = "درگاه پرداخت"

	btnSub   = "sub"
	btnSubFa = "لینک اشتراک"

	btnHelp   = "help"
	btnHelpFa = "راهنمای اتصال (آیفون، اندروید، غیره)"

	btnSupportFa = "چت با پشتیبانی"
)
const packagePrefix = "package-"

var backButton = tele.Btn{Unique: "back-btn", Text: "⬅️ بازگشت"}

func (b *BotHandler) buildMainMenu(middlewares ...tele.MiddlewareFunc) {
	mainMenu := &tele.ReplyMarkup{}

	purchaseBtn := mainMenu.Data(btnPurchaseSubFa, btnPurchaseSub)
	b.bot.Handle(&purchaseBtn, b.Purchase, middlewares...)

	usageBtn := mainMenu.Data(btnTrafficUsageFa, btnTrafficUsage)
	b.bot.Handle(&usageBtn, b.TrafficUsage, middlewares...)

	subBtn := mainMenu.Data(btnSubFa, btnSub)
	b.bot.Handle(&subBtn, b.Sub, middlewares...)

	helpBtn := mainMenu.Data(btnHelpFa, btnHelp)
	b.bot.Handle(&helpBtn, b.Help, middlewares...)

	supportBtn := mainMenu.URL(btnSupportFa, "https://t.me/zs_sup")
	b.bot.Handle(&supportBtn, b.MainMenu)

	b.bot.Handle(&backButton, b.MainMenu)

	r0 := mainMenu.Row(purchaseBtn)
	r1 := mainMenu.Row(usageBtn, subBtn)
	r2 := mainMenu.Row(supportBtn, helpBtn)
	mainMenu.Inline(r0, r1, r2)

	b.menu = mainMenu
}

func (b *BotHandler) buildPurchaseMenu(packages []*models.Package, middlewares ...tele.MiddlewareFunc) {
	purchaseMenu := &tele.ReplyMarkup{}

	availablePackages := make(map[string]*models.Package)

	var btns []tele.Btn
	for _, p := range packages {
		if p.Price == 0 { // We don't want to show free packages to users
			continue
		}
		uniqueName := packagePrefix + strconv.FormatInt(p.ID, 10)
		availablePackages[uniqueName] = p
		btn := purchaseMenu.Data(p.Name, uniqueName)
		b.bot.Handle(&btn, b.ChoosePurchaseMethod, middlewares...)
		btns = append(btns, btn)
	}

	rows := make([]tele.Row, 2+len(btns)/3)
	for i, btn := range btns {
		i /= 3
		rows[i] = append(rows[i], btn)
	}
	rows = append(rows, purchaseMenu.Row(backButton))
	purchaseMenu.Inline(rows...)
	b.purchaseMenu = purchaseMenu
	b.packages = availablePackages
}

func (b *BotHandler) buildPurchaseMethodMenu(middlewares ...tele.MiddlewareFunc) {
	purchaseMethod := &tele.ReplyMarkup{}
	btnNextPay := purchaseMethod.Data(btnPurchaseNextpayFa, btnPurchaseNextpay)
	btnCard2Card := purchaseMethod.Data(btnPurchaseCard2CardFa, btnPurchaseCard2Card)
	b.bot.Handle(&btnNextPay, b.HandleBankOrder, middlewares...)
	b.bot.Handle(&btnCard2Card, b.HandleCard2CardOrder, middlewares...)

	purchaseMethod.Inline(purchaseMethod.Row(btnCard2Card, btnNextPay), purchaseMethod.Row(backButton))
	b.purchaseMethodMenu = purchaseMethod
}

func (b *BotHandler) buildUsageMenu(back tele.Btn, middlewares ...tele.MiddlewareFunc) {
	usageMenu := &tele.ReplyMarkup{}
	usageMenu.Inline(usageMenu.Row(back))
	b.usageMenu = usageMenu
}

func (b *BotHandler) buildSubMenu(back tele.Btn) {
	subMenu := &tele.ReplyMarkup{}
	subMenu.Inline(subMenu.Row(back))
	b.subMenu = subMenu
}

func (b *BotHandler) buildHelpMenu(back tele.Btn) {
	helpMenu := &tele.ReplyMarkup{}
	helpMenu.Inline(helpMenu.Row(back))
	b.helpMenu = helpMenu
}

func (b *BotHandler) buildAdminConfirmationMenu() {
	var adminPurchaseConfirmationMenu = &tele.ReplyMarkup{}
	approve := adminPurchaseConfirmationMenu.Data("Approve", "approve-purchase")
	reject := adminPurchaseConfirmationMenu.Data("Reject", "reject-purchase")
	adminPurchaseConfirmationMenu.Inline(adminPurchaseConfirmationMenu.Row(approve, reject))

	b.bot.Handle(&approve, b.ApprovePurchase)
	b.bot.Handle(&reject, b.RejectPurchase)

	b.purchaseConfirmMenu = adminPurchaseConfirmationMenu
}
