package api

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/steeve/pulsar/config"
	"github.com/steeve/pulsar/xbmc"
)

func ClearCache(ctx *gin.Context) {
	os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
	xbmc.Notify("Pulsar", "Cache cleared", config.AddonIcon())
}
