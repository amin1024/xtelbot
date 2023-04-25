package xpanels

import (
	"database/sql"
	"embed"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"time"
)

type RenovateRule struct {
	Id       uint64 `db:"id"`
	Remark   string `db:"remark"`
	OldValue string `db:"old_value"`
	NewValue string `db:"new_value"`
	Ignore   bool   `db:"ignore"`
}

type KeyVal struct {
	Key string `db:"key"`
	Val string `db:"value"`
}

type User struct {
	Id             uint64         `db:"id"`
	Uuid           string         `db:"uuid"`
	Name           string         `db:"name"`
	LastOnline     string         `db:"last_online"`
	ExpiryTime     sql.NullString `db:"expiry_time"`
	UsageLimitGB   float32        `db:"usage_limit_GB"`
	CurrentUsageGB float32        `db:"current_usage_GB"`
}

type IHiddifyPanelRepo interface {
	ListRenovateRules() ([]RenovateRule, error)
	GetGroupedRules() (map[string][]RenovateRule, error)
	GetUser(uuid string) (User, error)
	InsertUser(uid, username, expireTime, startDate, mode string, lastOnline time.Time, trafficAllowed float32, packageDays int64) error
	InsertRenovateRule(rule RenovateRule) error
	GetDomains() ([]string, error)
	GetStrConfig() (map[string]string, error)
}

type HiddifyPanelRepo struct {
	db *sqlx.DB
}

//go:embed migrations/*.sql
var embedFs embed.FS

func (r *HiddifyPanelRepo) migrate() {
	goose.SetBaseFS(embedFs)
	if err := goose.SetDialect("sqlite3"); err != nil {
		panic(err)
	}
	if err := goose.Up(r.db.DB, "migrations"); err != nil {
		panic(err)
	}
}

func (r *HiddifyPanelRepo) ListRenovateRules() ([]RenovateRule, error) {
	var res []RenovateRule
	if err := r.db.Select(&res, "SELECT * FROM renovate_rule order by remark;"); err != nil {
		return res, err
	}
	return res, nil
}

// GetGroupedRules returns a rules grouped by their remark name.
func (r *HiddifyPanelRepo) GetGroupedRules() (map[string][]RenovateRule, error) {
	rules, err := r.ListRenovateRules()
	if err != nil {
		return nil, err
	}
	grouped := make(map[string][]RenovateRule)
	for _, r := range rules {
		_, ok := grouped[r.Remark]
		if ok {
			grouped[r.Remark] = append(grouped[r.Remark], r)
		}
		grouped[r.Remark] = []RenovateRule{r}
	}
	return grouped, nil
}

func (r *HiddifyPanelRepo) GetUser(uid string) (User, error) {
	var u User
	if err := r.db.Unsafe().Get(&u, "SELECT * FROM user WHERE  uuid = ?", uid); err != nil {
		return u, err
	}
	return u, nil
}

func (r *HiddifyPanelRepo) InsertUser(
	uid, username, expireTime, startDate, mode string, lastOnline time.Time,
	trafficAllowed float32, packageDays int64,
) error {
	q := `INSERT INTO user
    (uuid, name, last_online, expiry_time, usage_limit_GB, package_days, mode, start_date, current_usage_GB)
	values(?, ?, ?, ?, ?, ?, ?, ?, ?);`
	_, err := r.db.Exec(
		q,
		uid, username, lastOnline, expireTime, trafficAllowed, packageDays,
		mode, startDate, 0,
	)
	return err
}

func (r *HiddifyPanelRepo) InsertRenovateRule(rule RenovateRule) error {
	q := `INSERT INTO renovate_rule(remark, old_value, new_value, ignore) values(?, ?, ?, ?)
			ON CONFLICT (remark, old_value) DO UPDATE SET new_value = ?, ignore = ?;`
	_, err := r.db.Exec(q, rule.Remark, rule.OldValue, rule.NewValue, rule.Ignore, rule.NewValue, rule.Ignore)
	return err
}

func (r *HiddifyPanelRepo) GetDomains() ([]string, error) {
	// TODO: add a cache with timeout
	q := `SELECT domain FROM domain ORDER BY id LIMIT 3;`
	var res []string
	if err := r.db.Select(&res, q); err != nil {
		return res, err
	}
	return res, nil
}

func (r *HiddifyPanelRepo) GetStrConfig() (map[string]string, error) {
	// TODO: add a cache with timeout
	q := `SELECT key, value FROM str_config;`
	var data []KeyVal
	res := make(map[string]string)
	if err := r.db.Select(&data, q); err != nil {
		return res, err
	}
	for _, d := range data {
		res[d.Key] = d.Val
	}
	return res, nil
}
