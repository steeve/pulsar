package main

import (
	"net/http"
	"runtime"

	"github.com/steeve/pulsar/api"
	"github.com/steeve/pulsar/streamer"
)

func main() {
	// Make sure we are properly multithreaded.
	runtime.GOMAXPROCS(runtime.NumCPU())

	streamer.Start(streamer.StreamerConfiguration{
		LowerListenPort: 6889,
		UpperListenPort: 7000,
	})

	http.Handle("/", api.Routes())
	http.ListenAndServe(":8000", nil)
}
