package config

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/op/go-logging"
	"github.com/steeve/pulsar/xbmc"
)

var log = logging.MustGetLogger("config")

type Configuration struct {
	DownloadPath       string
	Info               *xbmc.AddonInfo
	Platform           *xbmc.Platform
	Language           string
	ProfilePath        string
	KeepFilesAfterStop bool
	UploadRateLimit    int
	DownloadRateLimit  int
	BTListenPortMin    int
	BTListenPortMax    int

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
		DownloadPath:       filepath.Dir(xbmc.GetSettingString("download_path")),
		Info:               info,
		Platform:           xbmc.GetPlatform(),
		Language:           xbmc.GetLanguage(xbmc.ISO_639_1),
		ProfilePath:        info.Profile,
		UploadRateLimit:    xbmc.GetSettingInt("max_upload_rate") * 1024,
		DownloadRateLimit:  xbmc.GetSettingInt("max_download_rate") * 1024,
		KeepFilesAfterStop: xbmc.GetSettingBool("keep_files"),
		BTListenPortMin:    xbmc.GetSettingInt("listen_port_min"),
		BTListenPortMax:    xbmc.GetSettingInt("listen_port_max"),
		SocksEnabled:       xbmc.GetSettingBool("socks_enabled"),
		SocksHost:          xbmc.GetSettingString("socks_host"),
		SocksPort:          xbmc.GetSettingInt("socks_port"),
		SocksLogin:         xbmc.GetSettingString("socks_login"),
		SocksPassword:      xbmc.GetSettingString("socks_password"),
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
