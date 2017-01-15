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
	"github.com/scakemyer/quasar/api"
	"github.com/scakemyer/quasar/bittorrent"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/util"
	"github.com/scakemyer/quasar/xbmc"
)

var log = logging.MustGetLogger("main")

const (
	QuasarLogo = `________
\_____  \  __ _______    ___________ _______
 /  / \  \|  |  \__  \  /  ___/\__  \\_  __ \
/   \_/.  \  |  // __ \_\___ \  / __ \|  | \/
\_____\ \_/____/(____  /____  >(____  /__|
       \__>          \/     \/      \/
`
)

func ensureSingleInstance() {
	http.Head(fmt.Sprintf("http://localhost:%d/shutdown", config.ListenPort))
}

func makeBTConfiguration(conf *config.Configuration) *bittorrent.BTConfiguration {
	btConfig := &bittorrent.BTConfiguration{
		SpoofUserAgent:      conf.SpoofUserAgent,
		BackgroundHandling:  conf.BackgroundHandling,
		BufferSize:          conf.BufferSize,
		MaxUploadRate:       conf.UploadRateLimit,
		MaxDownloadRate:     conf.DownloadRateLimit,
		LimitAfterBuffering: conf.LimitAfterBuffering,
		ConnectionsLimit:    conf.ConnectionsLimit,
		SessionSave:         conf.SessionSave,
		ShareRatioLimit:     conf.ShareRatioLimit,
		SeedTimeRatioLimit:  conf.SeedTimeRatioLimit,
		SeedTimeLimit:       conf.SeedTimeLimit,
		DisableDHT:          conf.DisableDHT,
		DisableUPNP:         conf.DisableUPNP,
		EncryptionPolicy:    conf.EncryptionPolicy,
		LowerListenPort:     conf.BTListenPortMin,
		UpperListenPort:     conf.BTListenPortMax,
		TunedStorage:        conf.TunedStorage,
		DownloadPath:        conf.DownloadPath,
		TorrentsPath:        conf.TorrentsPath,
	}

	if conf.SocksEnabled == true {
		btConfig.Proxy = &bittorrent.ProxySettings{
			Type:     conf.ProxyType,
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

	// logging.SetFormatter(logging.MustStringFormatter("%{time:2006-01-02 15:04:05}  %{level:.4s}  %{module:-15s}  %{message}"))
	logging.SetFormatter(logging.MustStringFormatter(
		`%{color}%{level:.4s}  %{module:-12s} ▶ %{shortfunc:-15s}  %{color:reset}%{message}`,
	))
	logging.SetBackend(logging.NewLogBackend(os.Stdout, "", 0))

	for _, line := range strings.Split(QuasarLogo, "\n") {
		log.Debug(line)
	}
	log.Infof("Version: %s Go: %s", util.Version[1:len(util.Version) - 1], runtime.Version())

	conf := config.Reload()

	log.Infof("Addon: %s v%s", conf.Info.Id, conf.Info.Version)

	ensureSingleInstance()
	Migrate()

	btService := bittorrent.NewBTService(*makeBTConfiguration(conf))

	var shutdown = func() {
		log.Info("Shutting down...")
		btService.Close()
		log.Info("Bye bye")
		os.Exit(0)
	}

	var watchParentProcess = func() {
		for {
			if os.Getppid() == 1 {
				log.Warning("Parent shut down, shutting down too...")
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

	xbmc.ResetRPC()

	log.Info("Updating Kodi add-on repositories...")
	xbmc.UpdateAddonRepos()

	xbmc.Notify("Quasar", "LOCALIZE[30208]", config.AddonIcon())

	http.ListenAndServe(":"+strconv.Itoa(config.ListenPort), nil)
}
