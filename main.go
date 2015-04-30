package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/op/go-logging"
	"github.com/steeve/pulsar/api"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/config"
	"github.com/steeve/pulsar/util"
	"github.com/steeve/pulsar/xbmc"
)

var log = logging.MustGetLogger("main")

const (
	PulsarLogo = `
             .__
______  __ __|  |   ___________ _______
\____ \|  |  \  |  /  ___/\__  \\_  __ \
|  |_> >  |  /  |__\___ \  / __ \|  | \/
|   __/|____/|____/____  >(____  /__|
|__|                   \/      \/
`
)

func ensureSingleInstance() {
	http.Head(fmt.Sprintf("http://localhost:%d/shutdown", config.ListenPort))
}

func main() {
	// Make sure we are properly multithreaded.
	runtime.GOMAXPROCS(runtime.NumCPU())

	logging.SetFormatter(logging.MustStringFormatter("%{time:2006-01-02 15:04:05}  %{level:.4s}  %{module:-15s}  %{message}"))
	logging.SetBackend(logging.NewLogBackend(os.Stdout, "", 0))

	for _, line := range strings.Split(PulsarLogo, "\n") {
		log.Info(line)
	}
	log.Info("Version: %s Git: %s Go: %s", util.Version, util.GitCommit, runtime.Version())

	config.Reload()

	ensureSingleInstance()
	Migrate()

	xbmc.CloseAllDialogs()

	log.Info("Addon: %s v%s", config.Get().Info.Id, config.Get().Info.Version)

	btService := bittorrent.NewBTService(bittorrent.BTConfiguration{
		LowerListenPort: config.Get().BTListenPortMin,
		UpperListenPort: config.Get().BTListenPortMax,
		DownloadPath:    config.Get().DownloadPath,
		MaxUploadRate:   config.Get().UploadRateLimit,
		MaxDownloadRate: config.Get().DownloadRateLimit,
	})

	var shutdown = func() {
		log.Info("Shutting down...")
		btService.Close()
		log.Info("Bye bye")
		os.Exit(0)
	}

	var watchParentProcess = func() {
		for {
			// did the parent die? shutdown!
			if os.Getppid() == 1 {
				log.Warning("Parent shut down. Me too.")
				go shutdown()
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
	go watchParentProcess()

	http.Handle("/", api.Routes(btService))
	http.Handle("/files/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := http.StripPrefix("/files/", http.FileServer(bittorrent.NewTorrentFS(btService, config.Get().DownloadPath)))
		handler.ServeHTTP(w, r)
	}))
	http.Handle("/reload", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config.Reload()
		btService.Reconfigure(bittorrent.BTConfiguration{
			LowerListenPort: config.Get().BTListenPortMin,
			UpperListenPort: config.Get().BTListenPortMax,
			DownloadPath:    config.Get().DownloadPath,
			MaxUploadRate:   config.Get().UploadRateLimit,
			MaxDownloadRate: config.Get().DownloadRateLimit,
		})
	}))
	http.Handle("/shutdown", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shutdown()
	}))

	xbmc.Notify("Pulsar", "Pulsar daemon has started")

	http.ListenAndServe(":"+strconv.Itoa(config.ListenPort), nil)
}
