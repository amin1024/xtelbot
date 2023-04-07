package xpanels

import (
	"context"
	"github.com/amin1024/xtelbot/conf"
	handlerCommand "github.com/xtls/xray-core/app/proxyman/command"
	statsCommand "github.com/xtls/xray-core/app/stats/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vmess"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"regexp"
	"strings"
)

type XClient struct {
	Uuid    string
	Email   string
	Level   uint32
	AlterId uint32
}

type DownUpStat struct {
	Downlink int64
	Uplink   int64
}

// -----------------------------------------------------------------

type XrayService struct {
	Addr string

	handler handlerCommand.HandlerServiceClient
	stats   statsCommand.StatsServiceClient

	log *zap.SugaredLogger
}

func NewXrayService(addr string) *XrayService {
	log := conf.NewLogger()
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	h := handlerCommand.NewHandlerServiceClient(conn)
	s := statsCommand.NewStatsServiceClient(conn)
	return &XrayService{
		Addr:    addr,
		handler: h,
		stats:   s,
		log:     log,
	}
}

func (x *XrayService) Restart() error {
	x.log.Error("Not implemented yet")
	return nil
}

func (x *XrayService) GetInboundStats() (map[string]DownUpStat, error) {
	var inboundRegex = regexp.MustCompile("^inbound>>>([^>]+)>>>traffic>>>(uplink|downlink)$")
	result := make(map[string]DownUpStat)

	r, err := x.stats.QueryStats(context.Background(), &statsCommand.QueryStatsRequest{
		Pattern: "",
		Reset_:  false,
	})
	if err != nil {
		return nil, err
	}

	for _, stat := range r.GetStat() {
		matches := inboundRegex.FindStringSubmatch(stat.GetName())
		inboundTag := matches[1]
		m, ok := result[inboundTag]
		if !ok {
			result[inboundTag] = DownUpStat{}
			m = result[inboundTag]
		}
		if matches[2] == "downlink" {
			m.Downlink = stat.GetValue()
		} else {
			m.Uplink = stat.GetValue()
		}
	}

	return result, nil
}

func (x *XrayService) AddClient(c XClient, inboundTag string) error {
	// Detect the inbound type based on its name
	// It is just a hack around it! there should be a better way!
	if strings.Contains(inboundTag, "vmess") {
		return x.addVmessAccount(c, inboundTag)
	} else if strings.Contains(inboundTag, "vless") {
		return x.addVlessAccount(c, inboundTag)
	} else if strings.Contains(inboundTag, "trojan") {
		return x.addTrojanAccount(c, inboundTag)
	}
	x.log.Warnw("XrayService.AddClient: Unsupported inbound", "tag", inboundTag)
	return nil
}

func (x *XrayService) addVmessAccount(c XClient, inboundTag string) error {
	addUserCmd := &handlerCommand.AddUserOperation{
		User: &protocol.User{
			Level: c.Level,
			Email: c.Email,
			Account: serial.ToTypedMessage(&vmess.Account{
				Id:      c.Uuid,
				AlterId: c.AlterId,
			}),
		},
	}

	_, err := x.handler.AlterInbound(context.Background(), &handlerCommand.AlterInboundRequest{
		Tag:       inboundTag,
		Operation: serial.ToTypedMessage(addUserCmd),
	})
	return err
}

func (x *XrayService) addVlessAccount(c XClient, inboundTag string) error {
	addUserCmd := &handlerCommand.AddUserOperation{
		User: &protocol.User{
			Level: c.Level,
			Email: c.Email,
			Account: serial.ToTypedMessage(&vless.Account{
				Id:         c.Uuid,
				Encryption: "none",
			}),
		},
	}

	_, err := x.handler.AlterInbound(context.Background(), &handlerCommand.AlterInboundRequest{
		Tag:       inboundTag,
		Operation: serial.ToTypedMessage(addUserCmd),
	})
	return err
}

func (x *XrayService) addTrojanAccount(c XClient, inboundTag string) error {
	addUserOp := &handlerCommand.AddUserOperation{
		User: &protocol.User{
			Level: c.Level,
			Email: c.Email,
			Account: serial.ToTypedMessage(&trojan.Account{
				Password: c.Uuid,
			}),
		},
	}
	_, err := x.handler.AlterInbound(context.Background(), &handlerCommand.AlterInboundRequest{
		Tag:       inboundTag,
		Operation: serial.ToTypedMessage(addUserOp),
	})
	return err
}
