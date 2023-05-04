package repo

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/amin1024/xtelbot/core/e"
	"github.com/amin1024/xtelbot/core/repo/models"
	"github.com/friendsofgo/errors"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

func SaveOrUpdateUser(u *models.Tuser) error {
	err := u.Upsert(
		context.Background(), db, true,
		[]string{models.TuserColumns.Tid},
		boil.Whitelist(models.TuserColumns.Username),
		boil.Infer(),
	)
	return err
}

func UpdateUser(u *models.Tuser) error {
	rowsAffected, err := u.Update(context.Background(), db, boil.Infer())
	if err != nil {
		//TODO: wrap error
		return err
	}
	if rowsAffected != 1 {
		return fmt.Errorf("!wtf")
	}
	return nil
}

func GetUserByTid(uid uint64) (*models.Tuser, error) {
	u, err := models.Tusers(
		qm.Load(models.TuserRels.Package),
		qm.Where(models.TuserColumns.Tid+"=?", uid),
	).One(context.Background(), db)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return u, e.UserNotFound
		}
		return u, fmt.Errorf("%s: %w", err, e.BaseError)
	}
	//if !u.Active {
	//	return u, e.UserIsNotActive
	//}
	return u, nil
}

func GetUser(userId int64) (*models.Tuser, error) {
	u, err := models.Tusers(
		qm.Load(models.TuserRels.Package),
		qm.Where(models.TuserColumns.ID+"=?", userId),
		qm.And("active=?", true),
	).One(context.Background(), db)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return u, e.UserNotFound
		}
		return u, fmt.Errorf("%s: %w", err, e.BaseError)
	}
	return u, nil
}

func GetUserByToken(token string) (*models.Tuser, error) {
	u, err := models.Tusers(
		models.TuserWhere.Token.EQ(token),
		models.TuserWhere.Active.EQ(true),
	).One(context.Background(), db)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return u, e.UserNotFound
		}
		return u, fmt.Errorf("%s: %w", err, e.BaseError)
	}
	return u, nil
}

func GetAllUsers() ([]*models.Tuser, error) {
	// TODO: potential bug when the number of users grow, work around it later on
	users, err := models.Tusers(
		models.TuserWhere.Active.EQ(true),
	).All(context.Background(), db)

	return users, err
}
