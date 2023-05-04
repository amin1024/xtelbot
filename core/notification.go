package core

import "github.com/amin1024/xtelbot/core/repo/models"

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
