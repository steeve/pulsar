package api

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/pulsar/config"
	"github.com/scakemyer/pulsar/xbmc"
)

func ClearCache(ctx *gin.Context) {
	os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
	xbmc.Notify("Pulsar", "LOCALIZE[30200]", config.AddonIcon())
}
