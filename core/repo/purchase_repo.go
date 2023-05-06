package repo

import (
	"context"
	"database/sql"
	"github.com/amin1024/xtelbot/core/e"
	"github.com/amin1024/xtelbot/core/repo/models"
	"github.com/friendsofgo/errors"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"time"
)

type PurchaseStatus int64

const (
	PurchaseUnknown PurchaseStatus = iota
	PurchaseConfirmed
	PurchaseIsProcessing
	PurchaseRejected
	PurchaseCancelled
	PurchaseWaitingForBankCallback
)

func InsertPurchase(purchase *models.Purchase) error {
	return purchase.Insert(context.Background(), db, boil.Infer())
}

func LastPurchasesByUserId(uid int64, status PurchaseStatus) (*models.Purchase, error) {
	p, err := models.Purchases(
		models.PurchaseWhere.TuserID.EQ(uid),
		models.PurchaseWhere.Status.EQ(int64(status)),
		qm.OrderBy("created_at desc"),
	).One(context.Background(), db)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return p, e.PurchaseNotFound
	}
	return p, err
}

func GetPurchaseById(id int64) (*models.Purchase, error) {
	return models.Purchases(
		models.PurchaseWhere.ID.EQ(id),
		qm.Load(models.PurchaseRels.Package),
		qm.Load(models.PurchaseRels.Tuser),
	).One(context.Background(), db)
}

func UpdatePurchase(p *models.Purchase) error {
	_, err := p.Update(context.Background(), db, boil.Infer())
	return err
}

//
//func RemoveUnknownPurchases(user *models.Tuser) (int64, error) {
//	q := `UPDATE purchase SET status = ? WHERE status = ? and tuser_id = ? and id not in (SELECT max(id) FROM purchase WHERE status = ? and tuser_id = ?)`
//	query := qm.SQL(q, PurchaseCancelled, PurchaseUnknown, user.ID, PurchaseUnknown, user.ID)
//	res, err := models.Purchases(query).Exec(db)
//	if err != nil {
//		return 0, err
//	}
//	return res.RowsAffected()
//}

func SetPurchaseAsProcessing(purchase *models.Purchase) error {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	purchase.ProcessedAt = null.TimeFrom(time.Now())
	purchase.Status = int64(PurchaseIsProcessing)
	if _, err := purchase.Update(context.Background(), tx, boil.Infer()); err != nil {
		tx.Rollback()
		return err
	}
	// set all other unknown purchases as cancelled for this user
	//m := models.M{models.PurchaseColumns.Status: PurchaseCancelled}
	q := qm.SQL("UPDATE purchase SET status = ? WHERE status = ? and tuser_id = ?", int64(PurchaseCancelled), int64(PurchaseUnknown), purchase.TuserID)
	if _, err := models.Purchases(q).Exec(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// CreatePurchase creates a new order purchase and will cancel previous unprocessed purchases
func CreatePurchase(purchase *models.Purchase) error {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	if err := purchase.Insert(context.Background(), tx, boil.Infer()); err != nil {
		return err
	}
	purchase.ProcessedAt = null.TimeFrom(time.Now())
	// set all other unknown purchases as cancelled for this user
	q := qm.SQL("UPDATE purchase SET status = ? WHERE status = ? and tuser_id = ?", int64(PurchaseCancelled), int64(PurchaseUnknown), purchase.TuserID)
	if _, err := models.Purchases(q).Exec(tx); err != nil {
		return tx.Rollback()
	}
	return tx.Commit()
}
