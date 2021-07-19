package main

import (
	"log"
	"time"

	"github.com/mlveggo/qualisys/pkg/discover"
)

func main() {
	discovery := discover.NewDiscovery(4545, 1*time.Second)
	responses, err := discovery.Discover()
	if err != nil {
		log.Println(err)
		return
	}
	for _, response := range responses {
		log.Println(response)
	}
}
