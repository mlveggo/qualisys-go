qualisys-go
===========

Go sdk for Qualisys Track Manager streaming of motion capture data.

Discovery example
-----------------

```Go
package main

import (
	"log"
	"time"

	"github.com/mlveggo/qualisys-go/pkg/discover"
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
```

Issues
------

-	It should always fail to connect to older QTM version (Support 1.22 and upwards)
-	StreamFrames - Missing support for \[:channels] handling.
-	Makefile doesn't work on Windows (golangci-lint/goreview handling)
-	Support passing in an context.Context in NewProtocol()
