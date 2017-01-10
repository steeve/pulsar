package config

import (
	"os"
	"sync"
	"strings"
	"strconv"
	"path/filepath"

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
	SpoofUserAgent      int
	BackgroundHandling  bool
	KeepFilesAfterStop  bool
	KeepFilesAsk        bool
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
	ConnectionsLimit    int
	SessionSave         int
	ShareRatioLimit     int
	SeedTimeRatioLimit  int
	SeedTimeLimit       int
	DisableDHT          bool
	DisableUPNP         bool
	EncryptionPolicy    int
	BTListenPortMin     int
	BTListenPortMax     int
	TunedStorage        bool
	Scrobble            bool
	TraktUsername       string
	TraktToken          string
	TraktRefreshToken   string
	TvScraper           int
	IgnoreDuplicates    bool
	UseCloudHole        bool
	CloudHoleKey        string
	TMDBApiKey          string
	OSDBUser            string
	OSDBPass            string

	SortingModeMovies            int
	SortingModeShows             int
	ResolutionPreferenceMovies   int
	ResolutionPreferenceShows    int
	PercentageAdditionalSeeders  int

	CustomProviderTimeoutEnabled bool
	CustomProviderTimeout        int

	ProxyType     int
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
	info.TempPath = filepath.Join(xbmc.TranslatePath("special://temp"), "quasar")
	platform := xbmc.GetPlatform()

	os.RemoveAll(info.TempPath)
	os.MkdirAll(info.TempPath, 0777)

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

	xbmcSettings := xbmc.GetAllSettings()
	settings := make(map[string]interface{})
	for _, setting := range xbmcSettings {
		switch setting.Type {
		case "enum":
			fallthrough
		case "number":
			value, _ := strconv.Atoi(setting.Value)
			settings[setting.Key] = value
		case "bool":
			settings[setting.Key] = (setting.Value == "true")
		default:
			settings[setting.Key] = setting.Value
		}
	}

	newConfig := Configuration{
		DownloadPath:        downloadPath,
		LibraryPath:         filepath.Dir(settings["library_path"].(string)),
		TorrentsPath:        filepath.Join(downloadPath, "Torrents"),
		Info:                info,
		Platform:            platform,
		Language:            xbmc.GetLanguageISO_639_1(),
		ProfilePath:         info.Profile,
		BufferSize:          settings["buffer_size"].(int) * 1024 * 1024,
		UploadRateLimit:     settings["max_upload_rate"].(int) * 1024,
		DownloadRateLimit:   settings["max_download_rate"].(int) * 1024,
		SpoofUserAgent:      settings["spoof_user_agent"].(int),
		LimitAfterBuffering: settings["limit_after_buffering"].(bool),
		BackgroundHandling:  settings["background_handling"].(bool),
		KeepFilesAfterStop:  settings["keep_files"].(bool),
		KeepFilesAsk:        settings["keep_files_ask"].(bool),
		ResultsPerPage:      settings["results_per_page"].(int),
		EnableOverlayStatus: settings["enable_overlay_status"].(bool),
		ChooseStreamAuto:    settings["choose_stream_auto"].(bool),
		UseOriginalTitle:    settings["use_original_title"].(bool),
		AddSpecials:         settings["add_specials"].(bool),
		PreReleaseUpdates:   settings["pre_release_updates"].(bool),
		ShareRatioLimit:     settings["share_ratio_limit"].(int),
		SeedTimeRatioLimit:  settings["seed_time_ratio_limit"].(int),
		SeedTimeLimit:       settings["seed_time_limit"].(int) * 3600,
		DisableDHT:          settings["disable_dht"].(bool),
		DisableUPNP:         settings["disable_upnp"].(bool),
		EncryptionPolicy:    settings["encryption_policy"].(int),
		BTListenPortMin:     settings["listen_port_min"].(int),
		BTListenPortMax:     settings["listen_port_max"].(int),
		TunedStorage:        settings["tuned_storage"].(bool),
		ConnectionsLimit:    settings["connections_limit"].(int),
		SessionSave:         settings["session_save"].(int),
		Scrobble:            settings["trakt_scrobble"].(bool),
		TraktUsername:       settings["trakt_username"].(string),
		TraktToken:          settings["trakt_token"].(string),
		TraktRefreshToken:   settings["trakt_refresh_token"].(string),
		IgnoreDuplicates:    settings["library_ignore_duplicates"].(bool),
		TvScraper:           settings["library_tv_scraper"].(int),
		UseCloudHole:        settings["use_cloudhole"].(bool),
		CloudHoleKey:        settings["cloudhole_key"].(string),
		TMDBApiKey:          settings["tmdb_api_key"].(string),
		OSDBUser:            settings["osdb_user"].(string),
		OSDBPass:            settings["osdb_pass"].(string),

		SortingModeMovies:            settings["sorting_mode_movies"].(int),
		SortingModeShows:             settings["sorting_mode_shows"].(int),
		ResolutionPreferenceMovies:   settings["resolution_preference_movies"].(int),
		ResolutionPreferenceShows:    settings["resolution_preference_shows"].(int),
		PercentageAdditionalSeeders:  settings["percentage_additional_seeders"].(int),

		CustomProviderTimeoutEnabled: settings["custom_provider_timeout_enabled"].(bool),
		CustomProviderTimeout:        settings["custom_provider_timeout"].(int),

		ProxyType:     settings["proxy_type"].(int),
		SocksEnabled:  settings["socks_enabled"].(bool),
		SocksHost:     settings["socks_host"].(string),
		SocksPort:     settings["socks_port"].(int),
		SocksLogin:    settings["socks_login"].(string),
		SocksPassword: settings["socks_password"].(string),
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
