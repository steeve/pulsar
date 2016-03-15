package config

import (
	"path/filepath"
	"strings"
	"sync"
	"os"

	"github.com/op/go-logging"
	"github.com/scakemyer/quasar/xbmc"
)

var log = logging.MustGetLogger("config")

type Configuration struct {
	DownloadPath        string
	TorrentsPath        string
	LibraryPath         string
	Info                *xbmc.AddonInfo
	Platform            *xbmc.Platform
	Language            string
	ProfilePath         string
	BackgroundHandling  bool
	KeepFilesAfterStop  bool
	ResultsPerPage      int
	EnableOverlayStatus bool
	ChooseStreamAuto    bool
	UseOriginalTitle    bool
	AddSpecials         bool
	PreReleaseUpdates   bool
	BufferSize          int
	UploadRateLimit     int
	DownloadRateLimit   int
	LimitAfterBuffering bool
	ShareRatioLimit     int
	SeedTimeRatioLimit  int
	SeedTimeLimit       int
	DisableDHT          bool
	BTListenPortMin     int
	BTListenPortMax     int
	ConnectionsLimit    int
	SessionSave         int
	Scrobble            bool
	TraktUsername       string
	TraktToken          string
	TraktRefreshToken   string
	CloudHoleKey        string
	TMDBApiKey          string

	SortingModeMovies            int
	SortingModeShows             int
	ResolutionPreferenceMovies   int
	ResolutionPreferenceShows    int
	PercentageAdditionalSeeders  int

	CustomProviderTimeoutEnabled bool
	CustomProviderTimeout        int

	SocksEnabled  bool
	SocksHost     string
	SocksPort     int
	SocksLogin    string
	SocksPassword string
}

var config = &Configuration{}
var lock = sync.RWMutex{}

const (
	ListenPort = 65251
)

func Get() *Configuration {
	lock.RLock()
	defer lock.RUnlock()
	return config
}

func Reload() *Configuration {
	log.Info("Reloading configuration...")

	info := xbmc.GetAddonInfo()
	info.Path = xbmc.TranslatePath(info.Path)
	info.Profile = xbmc.TranslatePath(info.Profile)
	platform := xbmc.GetPlatform()

	if platform.OS == "android" {
		legacyPath := strings.Replace(info.Path, "/storage/emulated/0", "/storage/emulated/legacy", 1)
		if _, err := os.Stat(legacyPath); err == nil {
			info.Path = legacyPath
			info.Profile = strings.Replace(info.Profile, "/storage/emulated/0", "/storage/emulated/legacy", 1)
			log.Info("Using /storage/emulated/legacy path.")
		}
	}

	downloadPath := filepath.Dir(xbmc.GetSettingString("download_path"))
	if downloadPath == "." {
		xbmc.Notify("Quasar", "LOCALIZE[30113]", filepath.Join(info.Path, "icon.png"))
		xbmc.AddonSettings("plugin.video.quasar")
	}

	newConfig := Configuration{
		DownloadPath:        downloadPath,
		LibraryPath:         filepath.Dir(xbmc.GetSettingString("library_path")),
		TorrentsPath:        filepath.Join(downloadPath, "Torrents"),
		Info:                info,
		Platform:            platform,
		Language:            xbmc.GetLanguageISO_639_1(),
		ProfilePath:         info.Profile,
		BufferSize:          xbmc.GetSettingInt("buffer_size") * 1024 * 1024,
		UploadRateLimit:     xbmc.GetSettingInt("max_upload_rate") * 1024,
		DownloadRateLimit:   xbmc.GetSettingInt("max_download_rate") * 1024,
		LimitAfterBuffering: xbmc.GetSettingBool("limit_after_buffering"),
		BackgroundHandling:  xbmc.GetSettingBool("background_handling"),
		KeepFilesAfterStop:  xbmc.GetSettingBool("keep_files"),
		ResultsPerPage:      xbmc.GetSettingInt("results_per_page"),
		EnableOverlayStatus: xbmc.GetSettingBool("enable_overlay_status"),
		ChooseStreamAuto:    xbmc.GetSettingBool("choose_stream_auto"),
		UseOriginalTitle:    xbmc.GetSettingBool("use_original_title"),
		AddSpecials:         xbmc.GetSettingBool("add_specials"),
		PreReleaseUpdates:   xbmc.GetSettingBool("pre_release_updates"),
		ShareRatioLimit:     xbmc.GetSettingInt("share_ratio_limit"),
		SeedTimeRatioLimit:  xbmc.GetSettingInt("seed_time_ratio_limit"),
		SeedTimeLimit:       xbmc.GetSettingInt("seed_time_limit") * 3600,
		DisableDHT:          xbmc.GetSettingBool("disable_dht"),
		BTListenPortMin:     xbmc.GetSettingInt("listen_port_min"),
		BTListenPortMax:     xbmc.GetSettingInt("listen_port_max"),
		ConnectionsLimit:    xbmc.GetSettingInt("connections_limit"),
		SessionSave:         xbmc.GetSettingInt("session_save"),
		Scrobble:            xbmc.GetSettingBool("trakt_scrobble"),
		TraktUsername:       xbmc.GetSettingString("trakt_username"),
		TraktToken:          xbmc.GetSettingString("trakt_token"),
		TraktRefreshToken:   xbmc.GetSettingString("trakt_refresh_token"),
		CloudHoleKey:        xbmc.GetSettingString("cloudhole_key"),
		TMDBApiKey:          xbmc.GetSettingString("tmdb_api_key"),

		SortingModeMovies:            xbmc.GetSettingInt("sorting_mode_movies"),
		SortingModeShows:             xbmc.GetSettingInt("sorting_mode_shows"),
		ResolutionPreferenceMovies:   xbmc.GetSettingInt("resolution_preference_movies"),
		ResolutionPreferenceShows:    xbmc.GetSettingInt("resolution_preference_shows"),
		PercentageAdditionalSeeders:  xbmc.GetSettingInt("percentage_additional_seeders"),

		CustomProviderTimeoutEnabled: xbmc.GetSettingBool("custom_provider_timeout_enabled"),
		CustomProviderTimeout:        xbmc.GetSettingInt("custom_provider_timeout"),

		SocksEnabled:  xbmc.GetSettingBool("socks_enabled"),
		SocksHost:     xbmc.GetSettingString("socks_host"),
		SocksPort:     xbmc.GetSettingInt("socks_port"),
		SocksLogin:    xbmc.GetSettingString("socks_login"),
		SocksPassword: xbmc.GetSettingString("socks_password"),
	}

	lock.Lock()
	config = &newConfig
	lock.Unlock()

	return config
}

func AddonIcon() string {
	return filepath.Join(Get().Info.Path, "icon.png")
}

func AddonResource(args ...string) string {
	return filepath.Join(Get().Info.Path, "resources", filepath.Join(args...))
}
