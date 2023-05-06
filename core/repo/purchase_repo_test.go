package repo

import (
	"github.com/amin1024/xtelbot/core/repo/models"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSetPurchaseAsProcessing(t *testing.T) {
	tearDown()
	users := populateUsers()
	// add some sample purchase

	pck, _ := GetPackage("")
	p1 := &models.Purchase{
		TuserID:     users[0].ID,
		PackageID:   pck.ID,
		Price:       pck.Price,
		PackageName: pck.Name,
		Status:      int64(PurchaseConfirmed),
	}
	p2 := &models.Purchase{
		TuserID:     users[0].ID,
		PackageID:   pck.ID,
		Price:       pck.Price,
		PackageName: pck.Name,
		Status:      int64(PurchaseUnknown),
	}
	p3 := &models.Purchase{
		TuserID:     users[0].ID,
		PackageID:   pck.ID,
		Price:       pck.Price,
		PackageName: pck.Name,
		Status:      int64(PurchaseUnknown),
	}
	p21 := &models.Purchase{
		TuserID:     users[1].ID,
		PackageID:   pck.ID,
		Price:       pck.Price,
		PackageName: pck.Name,
		Status:      int64(PurchaseUnknown),
	}
	InsertPurchase(p1)
	InsertPurchase(p2)
	InsertPurchase(p3)
	InsertPurchase(p21)

	user0Purchase, err := LastPurchasesByUserId(users[0].ID, PurchaseConfirmed)
	assert.NoError(t, err)
	assert.Equal(t, user0Purchase.ID, p1.ID)
	user0Purchase, err = LastPurchasesByUserId(users[0].ID, PurchaseUnknown)
	assert.NoError(t, err)
	assert.Equal(t, user0Purchase.ID, p3.ID)

	// Remove users[0] purchases, exactly 1 of them must be cancelled
	err = SetPurchaseAsProcessing(user0Purchase)
	assert.NoError(t, err)

	// assert if the correct purchase has been set as processing
	newUser0Purchase, err := GetPurchaseById(user0Purchase.ID)
	assert.NoError(t, err)
	assert.Equal(t, newUser0Purchase.ID, user0Purchase.ID)
	assert.Equal(t, int64(PurchaseIsProcessing), newUser0Purchase.Status)
	// assert other purchases has been cancelled
	newP2, err := GetPurchaseById(p2.ID)
	assert.NoError(t, err)
	assert.Equal(t, newP2.ID, p2.ID)
	assert.Equal(t, int64(PurchaseCancelled), newP2.Status)
	newP1, err := GetPurchaseById(p1.ID)
	assert.NoError(t, err)
	assert.Equal(t, newP1.ID, p1.ID)
	assert.Equal(t, int64(PurchaseConfirmed), newP1.Status)

	//	assert user1 purchases is untouched
	newP21, err := GetPurchaseById(p21.ID)
	assert.NoError(t, err)
	assert.Equal(t, newP21.ID, p21.ID)
	assert.Equal(t, int64(PurchaseUnknown), newP21.Status)
}
