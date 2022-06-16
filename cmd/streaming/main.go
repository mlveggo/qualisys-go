package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/mlveggo/qualisys-go"
	"github.com/mlveggo/qualisys-go/pkg/discover"
)

func HandlePackets(p *qualisys.Packet) {
	switch p.Type {
	case qualisys.PacketTypeEvent:
		log.Println("Event:", p.Event)
		if p.Event == qualisys.EventTypeQTMShuttingDown {
			os.Exit(0)
		}
	case qualisys.PacketTypeData:
		for _, c := range p.Data.Components {
			log.Println("Frame:", strconv.Itoa(int(p.Data.Frame)), c)
		}
	}
}

func main() {
	ip := "127.0.0.1"
	basePort := qualisys.DefaultLittleEndianPort
	if len(os.Args) <= 1 {
		discovery := discover.NewDiscovery(4545, 1*time.Second)
		responses, err := discovery.Discover()
		if err == nil {
			for _, response := range responses {
				log.Println("Using the first found QTM: ", response)
				ip = response.Address
				basePort = response.BasePort
				break
			}
		}
	} else {
		ip = os.Args[1]
	}
	log.Println("Connecting to: ", ip)
	rt := qualisys.NewProtocol(ip, basePort)
	parametersFetched := false
	streaming := false
	for {
		if !rt.IsConnected() {
			if err := rt.Connect(); err != nil {
				log.Println(err)
				continue
			}
		}
		if !parametersFetched {
			_, err := rt.GetParameters(qualisys.ParameterType6D)
			if err != nil {
				log.Println(err)
				continue
			}
			parametersFetched = true
		}
		if !streaming {
			if err := rt.StreamFramesAll(qualisys.ComponentType6DEulerResidual); err != nil {
				log.Println(err)
				continue
			}
		}
		for {
			p, err := rt.Receive()
			if p.EndOfData() {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			if err != nil || p.Error() {
				log.Println(err)
				return
			}
			HandlePackets(p)
		}
	}
}
