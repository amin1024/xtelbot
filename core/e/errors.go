package e

import (
	"errors"
	"fmt"
)

var BaseError = errors.New("core error")

var AllxNodesDead = fmt.Errorf("user not found: %w", BaseError)

var (
	UserNotFound    = fmt.Errorf("user not found: %w", BaseError)
	UserIsNotActive = fmt.Errorf("user is not acrive: %w", BaseError)

	BankCardNotFound = errors.New("there must be at least 1 bank card defined")

	ReceiptPhotoWithoutActualPurchase = errors.New("received a photo while user does not have a purchase")

	InvalidPurchaseIdFormat = errors.New("invalid purchase id format")
	PurchaseNotFound        = errors.New("purchase not found")

	PackageUpgradeFailedByXNodes = errors.New("xnodes failed to upgrade user's package")
)
