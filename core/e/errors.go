package e

import (
	"errors"
	"fmt"
)

var BaseError = errors.New("core error")

var UserNotFound = fmt.Errorf("user not found: %w", BaseError)

var AllxNodesDead = fmt.Errorf("user not found: %w", BaseError)
