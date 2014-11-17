package config

import (
	"path/filepath"
	"sync"

	"github.com/op/go-logging"
	"github.com/steeve/pulsar/xbmc"
)

var log = logging.MustGetLogger("config")

type Configuration struct {
	DownloadPath       string
	Info               *xbmc.AddonInfo
	Platform           *xbmc.Platform
	ProfilePath        string
	KeepFilesAfterStop bool
	UploadRateLimit    int
	DownloadRateLimit  int
	BTListenPortMin    int
	BTListenPortMax    int
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

func Reload() error {
	log.Info("Reloading configuration...")

	info := xbmc.GetAddonInfo()
	info.Path = xbmc.TranslatePath(info.Path)
	info.Profile = xbmc.TranslatePath(info.Profile)
	newConfig := Configuration{
		DownloadPath:       filepath.Dir(xbmc.GetSettingString("download_path")),
		Info:               info,
		Platform:           xbmc.GetPlatform(),
		ProfilePath:        info.Profile,
		UploadRateLimit:    xbmc.GetSettingInt("max_upload_rate") * 1024,
		DownloadRateLimit:  xbmc.GetSettingInt("max_download_rate") * 1024,
		KeepFilesAfterStop: xbmc.GetSettingBool("keep_files"),
		BTListenPortMin:    xbmc.GetSettingInt("listen_port_min"),
		BTListenPortMax:    xbmc.GetSettingInt("listen_port_max"),
	}
	lock.Lock()
	config = &newConfig
	lock.Unlock()

	return nil
}
