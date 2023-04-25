package repo

import (
	"context"
	"github.com/amin1024/xtelbot/core/e"
	"github.com/amin1024/xtelbot/core/repo/models"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	SetupDb("test_db.db")
	AutoMigrate()
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		panic(err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		panic(err)
	}
	SetupPackage()
	code := m.Run()
	os.Exit(code)
}

// Populate tables with data
func populateUsers() []*models.Tuser {
	p, err := GetPackage("")
	if err != nil {
		panic(err)
	}

	tu := models.Tuser{
		Tid:               1,
		Username:          "fake_uid=1",
		UUID:              "fake_uuid__1",
		Token:             "fake_token__1",
		Active:            true,
		AddedToNodesCount: 3,
		TrafficUsage:      41.3,
		PackageID:         p.ID,
	}
	err = tu.Insert(context.Background(), db, boil.Infer())
	if err != nil {
		panic(err)
	}
	tu2 := models.Tuser{
		Tid:               2,
		Username:          "fake_uid=2",
		UUID:              "fake_uuid__2",
		Token:             "fake_token__2",
		Active:            false,
		AddedToNodesCount: 0,
		TrafficUsage:      0,
		PackageID:         p.ID,
	}
	err = tu2.Insert(context.Background(), db, boil.Infer())
	if err != nil {
		panic(err)
	}
	return []*models.Tuser{&tu, &tu2}
}

// Clear tables
func tearDown() {
	_, err := db.Exec("DELETE FROM tuser;")
	if err != nil {
		panic(err)
	}
}

func TestGetUser(t *testing.T) {
	tearDown()
	populateUsers()

	// test if no user found
	_, err := GetUser(111)
	assert.ErrorIs(t, err, e.UserNotFound)

	// add a deactivated user
	tu := models.Tuser{
		Tid:      111,
		Username: "fake_1",
		// TODO: why package_id = 0 does not return an error!?
		PackageID: 1,
	}
	assert.NoError(t, tu.Insert(context.Background(), db, boil.Infer()))

	// We expect GetUser to ignore the `tid=111` because it is not activated yet
	_, err = GetUser(111)
	assert.ErrorIs(t, err, e.UserNotFound)

	// Activate the user
	tu.Active = true
	assert.NoError(t, UpdateUser(&tu))

	u, err := GetUser(111)
	assert.NoError(t, err, e.UserNotFound)
	assert.Equal(t, uint64(111), u.Tid)
	//p, err := u.Package().One(context.Background(), db)
	//assert.NoError(t, err)
	//assert.Equal(t, float32(1), p.TrafficAllowed)
	assert.Equal(t, float32(5), u.R.Package.TrafficAllowed)
}

func TestRegisterMultipleTimes(t *testing.T) {
	// Test what happens when a user register more than 1 times.
	// We expect nothing has changed
	tearDown()
	users := populateUsers()

	u := users[0]
	p, _ := GetPackage("")

	u.Username = "NEW_USERNAME"
	assert.NoError(t, SaveOrUpdateUser(u))

	fromDb, _ := GetUser(u.Tid)
	assert.Equal(t, u.UUID, fromDb.UUID)
	assert.Equal(t, p.Name, fromDb.R.Package.Name)
	assert.Equal(t, u.ExpireAt, fromDb.ExpireAt)
	assert.Equal(t, fromDb.Username, "NEW_USERNAME")
}
