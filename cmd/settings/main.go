package main

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/mlveggo/qualisys-go"
	"github.com/mlveggo/qualisys-go/pkg/discover"
)

func main() {
	discovery := discover.NewDiscovery(4545, 1*time.Second)
	responses, err := discovery.Discover()
	ip := "127.0.0.1"
	basePort := qualisys.DefaultLittleEndianPort
	if err == nil {
		for _, response := range responses {
			log.Println("Using the first found QTM: ", response)
			ip = response.Address
			basePort = response.BasePort
			break
		}
	}
	rt := qualisys.NewRtProtocol(ip, basePort)
	log.Println("Connecting to:", ip)
	if err := rt.Connect(); err != nil {
		log.Println(err)
		return
	}
	xml, err := rt.GetParameters(qualisys.ParameterTypeImage)
	if err != nil {
		log.Println(err)
		return
	}
	if err := rt.TakeControl(""); err != nil {
		log.Println(err)
		return
	}
	// Note: This relies on QTM connection using RT version 1.22.
	xml = strings.Replace(xml, "<QTM_Parameters_Ver_1.22>", "", 1)
	xml = strings.Replace(xml, "</QTM_Parameters_Ver_1.22>", "", 1)
	xml = strings.Replace(xml, "<Enabled>false", "<Enabled>true", 10)
	if err := rt.SetParameters(xml); err != nil {
		log.Println()
	}
	if err := rt.StreamFramesAll(qualisys.ComponentTypeImage); err != nil {
		log.Println(err)
		return
	}
	for {
		p, err := rt.Receive()
		if p.EndOfData() {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if err != nil || p.Error() {
			log.Println(err)
			continue
		}
		for _, c := range p.Data.Components {
			log.Println("Frame:", strconv.Itoa(int(p.Data.Frame)), c)
		}
	}
}
