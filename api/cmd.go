package api

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/i96751414/pulsar/config"
	"github.com/i96751414/pulsar/xbmc"
)

func ClearCache(ctx *gin.Context) {
	os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
	xbmc.Notify("Pulsar", xbmc.GetLocalizedString(32000), config.AddonIcon())
}
