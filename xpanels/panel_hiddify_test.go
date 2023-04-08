package xpanels

import (
	"database/sql/driver"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/amin1024/xtelbot/pb"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"testing"
	"time"
)

func newMockedLog() *zap.SugaredLogger {
	var ml, _ = zap.NewDevelopment()
	return ml.Sugar()
}

type HiddifyDateString struct{}

func (a HiddifyDateString) Match(v driver.Value) bool {
	tvalue, ok := v.(string)
	if !ok {
		return false
	}
	if _, err := time.Parse("2006-01-02", tvalue); err != nil {
		return false
	}
	return true
}

func TestHiddifyPanel_Add2panel(t *testing.T) {
	cmd := pb.AddUserCmd{
		Tid:            1,
		TUsername:      "u1",
		Uuid:           "u1_UUID",
		TrafficAllowed: 4,
		ExpireAt:       "2023-10-10T15:04:05Z",
		PackageDays:    11,
		Mode:           "daily",
	}

	mdb, mock, err := sqlmock.New()
	assert.NoError(t, err)
	mock.ExpectExec("INSERT INTO user").
		WithArgs(cmd.Uuid, cmd.TUsername, sqlmock.AnyArg(), HiddifyDateString{}, cmd.TrafficAllowed, cmd.PackageDays, cmd.Mode, HiddifyDateString{}, 0).
		WillReturnResult(sqlmock.NewResult(1, 1))

	db := sqlx.NewDb(mdb, "sqlite3")
	defer db.Close()

	p := &HiddifyPanel{
		name: "test",
		db:   db,
		xray: nil,
		log:  newMockedLog(),
	}

	assert.NoError(t, p.add2panel(&cmd))
}
