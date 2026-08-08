// Command discover broadcasts for QTM instances on the local network.
package main

import (
	"flag"
	"log"
	"time"

	"github.com/mlveggo/qualisys-go/pkg/discover"
)

func main() {
	port := flag.Int("port", 4545, "local UDP port to receive discovery responses on")
	timeout := flag.Duration("timeout", 1*time.Second, "how long to wait for responses")
	flag.Parse()

	discovery := discover.NewDiscovery(uint16(*port), *timeout)
	responses, err := discovery.Discover()
	if err != nil {
		log.Fatal(err)
	}
	if len(responses) == 0 {
		log.Println("No QTM instances responded")
		return
	}
	for _, response := range responses {
		log.Println(response)
	}
}
