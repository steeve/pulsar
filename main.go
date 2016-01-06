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
	"github.com/i96751414/pulsar/api"
	"github.com/i96751414/pulsar/bittorrent"
	"github.com/i96751414/pulsar/config"
	"github.com/i96751414/pulsar/util"
	"github.com/i96751414/pulsar/xbmc"
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

func makeBTConfiguration(conf *config.Configuration) *bittorrent.BTConfiguration {
	btConfig := &bittorrent.BTConfiguration{
		LowerListenPort: conf.BTListenPortMin,
		UpperListenPort: conf.BTListenPortMax,
		DownloadPath:    conf.DownloadPath,
		MaxUploadRate:   conf.UploadRateLimit,
		MaxDownloadRate: conf.DownloadRateLimit,
	}

	if conf.SocksEnabled == true {
		btConfig.Proxy = &bittorrent.ProxySettings{
			Type:     bittorrent.ProxyTypeSocks5Password,
			Hostname: conf.SocksHost,
			Port:     conf.SocksPort,
			Username: conf.SocksLogin,
			Password: conf.SocksPassword,
		}
	}

	return btConfig
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

	conf := config.Reload()

	ensureSingleInstance()
	Migrate()

	xbmc.CloseAllDialogs()

	log.Info("Addon: %s v%s", conf.Info.Id, conf.Info.Version)

	btService := bittorrent.NewBTService(*makeBTConfiguration(conf))

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
		btService.Reconfigure(*makeBTConfiguration(config.Reload()))
	}))
	http.Handle("/shutdown", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shutdown()
	}))

	xbmc.Notify("Pulsar", "Pulsar daemon has started", config.AddonIcon())

	http.ListenAndServe(":"+strconv.Itoa(config.ListenPort), nil)
}
