package xpanels

import (
	"database/sql/driver"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/amin1024/xtelbot/pb"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"strings"
	"testing"
	"time"
)

// implements IHiddifyPanelRepo
type mockedRepo struct {
	db *sqlx.DB
}

func (r *mockedRepo) InsertRenovateRule(rule RenovateRule) error {
	return nil
}

func (r *mockedRepo) ListRenovateRules() ([]RenovateRule, error) {
	return nil, nil
}

func (r *mockedRepo) InsertUser(uid, username, expireTime, startDate, mode string, lastOnline time.Time, trafficAllowed float32, packageDays int64) error {
	return nil
}

func (r *mockedRepo) GetDomains() ([]string, error) {
	return nil, nil
}

func (r *mockedRepo) GetUser(uuid string) (User, error) {
	return User{}, nil
}

func (r *mockedRepo) GetStrConfig() (map[string]string, error) {
	return nil, nil
}

func (r *mockedRepo) UpdateUserPackage(uuid, expireTime, startDate, mode string, trafficAllowed float32, packageDays int64) error {
	return nil
}

func (r *mockedRepo) GetGroupedRules() (map[string][]RenovateRule, error) {
	return map[string][]RenovateRule{
		"tls_WS_1": []RenovateRule{
			RenovateRule{
				Id:       1,
				Remark:   "tls_WS_1",
				OldValue: ":443",
				NewValue: ":9999",
			},
		},
		"gg": []RenovateRule{
			RenovateRule{
				Id:       2,
				Remark:   "gg",
				OldValue: "host=api.google.info",
				NewValue: "host=gg.com",
			},
		},
		"tls_tcp_trojan": []RenovateRule{
			RenovateRule{
				Id:       3,
				Remark:   "tls_tcp_trojan",
				OldValue: "host=api.google.info",
				NewValue: "host=newHost.com",
			},
			RenovateRule{
				Id:       4,
				Remark:   "tls_tcp_trojan",
				OldValue: "@api.google.info",
				NewValue: "@newDomain.com",
			},
		},
	}, nil
}

// -----------------------------------------------------------------

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
		Tid:       1,
		TUsername: "u1",
		Uuid:      "u1_UUID",
		Package: &pb.Package{
			TrafficAllowed: 4,
			ExpireAt:       "2023-10-10T15:04:05Z",
			PackageDays:    11,
			Mode:           "daily",
		},
	}

	mdb, mock, err := sqlmock.New()
	assert.NoError(t, err)
	mock.ExpectExec("INSERT INTO user").
		WithArgs(cmd.Uuid, cmd.TUsername, sqlmock.AnyArg(), HiddifyDateString{}, cmd.Package.TrafficAllowed, cmd.Package.PackageDays, cmd.Package.Mode, HiddifyDateString{}, 0).
		WillReturnResult(sqlmock.NewResult(1, 1))

	db := sqlx.NewDb(mdb, "sqlite3")
	defer db.Close()

	p := &HiddifyPanel{
		name: "test",
		repo: &mockedRepo{db: db},
		xray: nil,
		log:  newMockedLog(),
	}

	assert.NoError(t, p.add2panel(&cmd))
}

func TestHiddifyPanel_renovateV2rayConfig(t *testing.T) {
	subContent := `
		# Hiddify auto ip: 1.1.1.1 DE 12222 unknown ERROR fullname=AMAZON default:smt
		####################################
		##  direct  api.google.info  http:80
		####################################
		####################################
		##  direct  api.google.info  tls:443
		####################################
		
		# tls_h2_WS_direct_vless api.google.info 
		vless://a038567c-e119-4111-a526-bc57a8185810@api.google.info:443?sni=api.google.info&type=ws&host=api.google.info#tls_WS_1
		
		# tls_h2_WS_direct_trojan api.google.info 
		trojan://a038567c-e119-4111-a526-bc57a8185810@api.google.info:443?sni=api.google.info&host=api.google.info#tls_WS_trojan
		
		# tls_h2_tcp_direct_trojan api.google.info 
		trojan://a038567c-e119-4111-a526-bc57a8185810@api.google.info:443?&sni=api.google.info&host=api.google.info#tls_tcp_trojan
		`
	mr := &mockedRepo{}
	rules, _ := mr.GetGroupedRules()
	p := &HiddifyPanel{
		name:      "test",
		repo:      mr,
		xray:      nil,
		log:       newMockedLog(),
		renovator: &RuleRenovator{groupRules: rules},
	}

	newSub, err := p.renovator.Renovate(strings.NewReader(subContent), "")
	assert.NoError(t, err)
	// assert there are exactly the same number of lines after renovation
	assert.Equal(t, strings.Contains(subContent, "\n"), strings.Contains(newSub, "\n"))
	// assert port in the first url has changed
	assert.Contains(t, newSub, "vless://a038567c-e119-4111-a526-bc57a8185810@api.google.info:9999?sni=api.google.info&type=ws&host=api.google.info#tls_WS_1")
	assert.NotContains(t, newSub, "gg.com")
	// assert the second url is untouched
	assert.Contains(t, newSub, "trojan://a038567c-e119-4111-a526-bc57a8185810@api.google.info:443?sni=api.google.info&host=api.google.info#tls_WS_trojan")
	//	assert domain and port is changed on third uri
	// trojan://a038567c-e119-4111-a526-bc57a8185810@newdomain.com:443?&sni=api.google.info&host=newHost.com#tls_tcp_trojan
	assert.Contains(t, newSub, "trojan://a038567c-e119-4111-a526-bc57a8185810@newDomain.com:443?&sni=api.google.info&host=newHost.com#tls_tcp_trojan")
}
