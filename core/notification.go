package core

import (
	"github.com/amin1024/xtelbot/core/repo/models"
	tele "gopkg.in/telebot.v3"
)

type NotificationType int

const (
	PurchaseSuccessful NotificationType = iota
	PurchaseRejected
	PurchaseFailed

	UserMaxTrafficReached
)

type Notification struct {
	Type  NotificationType
	User  *models.Tuser
	Extra interface{}
}

// AdminPurchaseNotify used to send notification to admins on new card2card event
type AdminPurchaseNotify struct {
	Tuser    *models.Tuser
	Purchase *models.Purchase
	Photo    *tele.Photo
}
