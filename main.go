package main

import (
	"net/http"
	"os"
	"runtime"

	"github.com/op/go-logging"
	"github.com/steeve/pulsar/api"
	"github.com/steeve/pulsar/bittorrent"
)

func main() {
	// Make sure we are properly multithreaded.
	runtime.GOMAXPROCS(runtime.NumCPU())

	logging.SetFormatter(logging.MustStringFormatter("[%{time:2006-01-02 15:04:05}] [%{module}] %{level} %{message}"))
	logging.SetBackend(logging.NewLogBackend(os.Stderr, "", 0))

	btService := bittorrent.NewBTService(bittorrent.BTConfiguration{
		LowerListenPort: 6889,
		UpperListenPort: 7000,
	})
	btService.Start()

	http.Handle("/", api.Routes(btService))
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(bittorrent.NewTorrentFS(btService, "/Users/steeve/projects/go/src/github.com/steeve/pulsar"))))

	http.ListenAndServe(":8000", nil)
}
