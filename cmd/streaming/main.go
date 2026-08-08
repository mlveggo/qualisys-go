// Command streaming connects to QTM and prints streamed data frames.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	qualisys "github.com/mlveggo/qualisys-go"
	"github.com/mlveggo/qualisys-go/pkg/discover"
)

func handlePacket(p *qualisys.Packet) bool {
	switch p.Type {
	case qualisys.PacketTypeEvent:
		log.Println("Event:", p.Event)
		if p.Event == qualisys.EventTypeQTMShuttingDown {
			return false
		}
	case qualisys.PacketTypeData:
		for _, c := range p.Data.Components {
			log.Printf("Frame %d: %v", p.Data.Frame, c)
		}
		// Components this build of the SDK does not recognise keep their raw
		// bytes rather than being discarded with the rest of the frame.
		for _, unknown := range p.Data.UnknownComponentTypes() {
			log.Printf("Frame %d: undecodable component type %d", p.Data.Frame, unknown)
		}
	}
	return true
}

func findQTM() (string, int) {
	discovery := discover.NewDiscovery(4545, 1*time.Second)
	responses, err := discovery.Discover()
	if err != nil {
		log.Println("discovery failed:", err)
		return "127.0.0.1", qualisys.DefaultBasePort
	}
	for _, response := range responses {
		log.Println("Using the first QTM found:", response)
		return response.Address, response.BasePort
	}
	return "127.0.0.1", qualisys.DefaultBasePort
}

func main() {
	addr := flag.String("addr", "", "QTM address; discovered by broadcast when empty")
	port := flag.Int("port", qualisys.DefaultBasePort, "QTM base port")
	useUDP := flag.Bool("udp", false, "stream data over UDP instead of the TCP control connection")
	channels := flag.String("analog-channels", "", "restrict analog streaming to these channels, e.g. 1,3,5-8")
	flag.Parse()

	ip, basePort := *addr, *port
	if ip == "" {
		ip, basePort = findQTM()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rt := qualisys.NewProtocol(ip, basePort)
	defer rt.Disconnect()

	log.Printf("Connecting to %s:%d", ip, basePort)
	if err := rt.Connect(); err != nil {
		log.Fatal(err)
	}
	major, minor := rt.Version()
	log.Printf("Connected using RT protocol version %d.%d", major, minor)

	if version, err := rt.GetQTMVersion(); err == nil {
		log.Println("QTM version:", version)
	}

	opts := qualisys.ComponentOptions{AnalogChannels: *channels}
	components := []qualisys.ComponentType{qualisys.ComponentType6DEulerResidual}

	receive := rt.Receive
	if *useUDP {
		udpPort, err := rt.EnableUDPStream(0)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Receiving data on UDP port %d", udpPort)
		if err := rt.StreamFramesUDP(qualisys.StreamRateTypeAllFrames, 0, udpPort, "", opts, components...); err != nil {
			log.Fatal(err)
		}
		receive = rt.ReceiveUDP
	} else {
		if err := rt.StreamFramesWithOptions(qualisys.StreamRateTypeAllFrames, 0, opts, components...); err != nil {
			log.Fatal(err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down")
			_ = rt.StreamFramesStop()
			return
		default:
		}

		p, err := receive()
		if err != nil {
			// A truncated packet desynchronises the stream; the connection has
			// to be rebuilt rather than limped along.
			if errors.Is(err, qualisys.ErrTruncated) {
				log.Println("stream desynchronised, reconnecting:", err)
				return
			}
			log.Println(err)
			return
		}
		if p.EndOfData() {
			continue
		}
		if !handlePacket(p) {
			return
		}
	}
}
