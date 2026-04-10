package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/256dpi/naos/pkg/connect"
)

var listen = flag.String("listen", ":8080", "The address to listen on.")
var token = flag.String("token", "", "The token required to access the hub.")
var verbose = flag.Bool("verbose", false, "Enable verbose per-device logging.")

func main() {
	flag.Parse()

	server := connect.NewServer()
	server.SetToken(*token)
	if *verbose {
		server.SetLogger(func(format string, args ...any) {
			log.Printf(format, args...)
		})
	}

	fmt.Printf("Listening on %s\n", *listen)
	if err := http.ListenAndServe(*listen, server); err != nil {
		panic(err)
	}
}
