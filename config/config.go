package config

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/op/go-logging"
	"github.com/scakemyer/quasar/xbmc"
)

var log = logging.MustGetLogger("config")

type Configuration struct {
	DownloadPath        string
	LibraryPath         string
	Info                *xbmc.AddonInfo
	Platform            *xbmc.Platform
	Language            string
	ProfilePath         string
	KeepFilesAfterStop  bool
	EnablePaging        bool
	EnableOverlayStatus bool
	ChooseStreamAuto    bool
	PreReleaseUpdates   bool
	BufferSize          int
	UploadRateLimit     int
	DownloadRateLimit   int
	LimitAfterBuffering bool
	BTListenPortMin     int
	BTListenPortMax     int

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

	info.Path = strings.Replace(info.Path, "/storage/emulated/0", "/storage/emulated/legacy", 1)
	info.Profile = strings.Replace(info.Profile, "/storage/emulated/0", "/storage/emulated/legacy", 1)

	newConfig := Configuration{
		DownloadPath:        filepath.Dir(xbmc.GetSettingString("download_path")),
		LibraryPath:         filepath.Dir(xbmc.GetSettingString("library_path")),
		Info:                info,
		Platform:            xbmc.GetPlatform(),
		Language:            xbmc.GetLanguageISO_639_1(),
		ProfilePath:         info.Profile,
		BufferSize:          xbmc.GetSettingInt("buffer_size") * 1024 * 1024,
		UploadRateLimit:     xbmc.GetSettingInt("max_upload_rate") * 1024,
		DownloadRateLimit:   xbmc.GetSettingInt("max_download_rate") * 1024,
		LimitAfterBuffering: xbmc.GetSettingBool("limit_after_buffering"),
		KeepFilesAfterStop:  xbmc.GetSettingBool("keep_files"),
		EnablePaging:        xbmc.GetSettingBool("enable_paging"),
		EnableOverlayStatus: xbmc.GetSettingBool("enable_overlay_status"),
		ChooseStreamAuto:    xbmc.GetSettingBool("choose_stream_auto"),
		PreReleaseUpdates:   xbmc.GetSettingBool("pre_release_updates"),
		BTListenPortMin:     xbmc.GetSettingInt("listen_port_min"),
		BTListenPortMax:     xbmc.GetSettingInt("listen_port_max"),

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
