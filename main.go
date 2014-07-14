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

	// uri := `magnet:?xt=urn:btih:e256280cf0dcb27ee1a3dc49b7fdd33ebf0c0f6c&dn=The+Hunger+Games%3A+Catching+Fire+%282013%29+720p+BrRip+x264+-+YIFY&tr=udp%3A%2F%2Ftracker.openbittorrent.com%3A80&tr=udp%3A%2F%2Ftracker.publicbt.com%3A80&tr=udp%3A%2F%2Ftracker.istole.it%3A6969&tr=udp%3A%2F%2Fopen.demonii.com%3A1337`
	// player := bittorrent.NewBTPlayer(btstreamer, uri)
	// player.Buffer()

	http.Handle("/", api.Routes(btService))
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(bittorrent.NewTorrentFS(btService, "/Users/steeve/projects/go/src/github.com/steeve/pulsar"))))

	http.ListenAndServe(":8000", nil)
}
