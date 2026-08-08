// Command settings connects to QTM, enables all image cameras and streams the
// resulting images.
package main

import (
	"flag"
	"log"
	"strings"
	"time"

	qualisys "github.com/mlveggo/qualisys-go"
	"github.com/mlveggo/qualisys-go/pkg/discover"
)

func main() {
	addr := flag.String("addr", "", "QTM address; discovered by broadcast when empty")
	port := flag.Int("port", qualisys.DefaultBasePort, "QTM base port")
	password := flag.String("password", "", "password for TakeControl")
	flag.Parse()

	ip, basePort := *addr, *port
	if ip == "" {
		ip, basePort = "127.0.0.1", qualisys.DefaultBasePort
		discovery := discover.NewDiscovery(4545, 1*time.Second)
		if responses, err := discovery.Discover(); err == nil {
			for _, response := range responses {
				log.Println("Using the first QTM found:", response)
				ip, basePort = response.Address, response.BasePort
				break
			}
		}
	}

	rt := qualisys.NewProtocol(ip, basePort)
	defer rt.Disconnect()

	log.Printf("Connecting to %s:%d", ip, basePort)
	if err := rt.Connect(); err != nil {
		log.Fatal(err)
	}
	major, minor := rt.Version()
	log.Printf("Connected using RT protocol version %d.%d", major, minor)

	xml, err := rt.GetParameters(qualisys.ParameterTypeImage)
	if err != nil {
		log.Fatal(err)
	}
	if err := rt.TakeControl(*password); err != nil {
		log.Fatal(err)
	}
	defer func() { _ = rt.ReleaseControl() }()

	// The settings root element is named after the negotiated protocol version,
	// so it must not be hard-coded. StripParametersElement removes whichever
	// version was actually agreed on.
	inner := rt.StripParametersElement(xml)
	inner = strings.ReplaceAll(inner, "<Enabled>false</Enabled>", "<Enabled>true</Enabled>")

	if err := rt.SetParameters(inner); err != nil {
		log.Fatal(err)
	}
	if err := rt.StreamFramesAll(qualisys.ComponentTypeImage); err != nil {
		log.Fatal(err)
	}

	for {
		p, err := rt.Receive()
		if err != nil {
			log.Fatal(err)
		}
		if p.EndOfData() {
			continue
		}
		for _, c := range p.Data.Components {
			log.Printf("Frame %d: %v", p.Data.Frame, c)
		}
	}
}
