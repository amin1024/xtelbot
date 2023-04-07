package xpanels

import (
	"github.com/amin1024/xtelbot/pb"
	"google.golang.org/grpc"
	"log"
	"net"
)

// -----------------------------------------------------------------

type IPanel interface {
	pb.XNodeGrpcServer
}

// -----------------------------------------------------------------

// PanelBuilder builds a panel (hiddify, xui, etc.) based on the arguments
func PanelBuilder(conf map[string]string) pb.XNodeGrpcServer {
	ptype, _ := conf["type"]
	xrayPort, _ := conf["xrayPort"]

	xs := NewXrayService("127.0.0.1:" + xrayPort)
	switch ptype {
	case "hiddify":
		return NewHiddifyPanel(xs, conf)
	case "xui":
		return NewXuiPanel(xs, conf)
	default:
		log.Fatal("requires a valid panel type: [hiddify, xui]")
		return nil
	}
}

func StartXPanel(conf map[string]string) {
	port, _ := conf["port"]
	l, err := net.Listen("tcp", "0.0.0.0:"+port)
	defer l.Close()
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer()

	panel := PanelBuilder(conf)
	pb.RegisterXNodeGrpcServer(grpcServer, panel)

	if err := grpcServer.Serve(l); err != nil {
		log.Fatal(err)
	}
}
