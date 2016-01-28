package api

import (
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/xbmc"
)

func ClearCache(ctx *gin.Context) {
	os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
	xbmc.Notify("Quasar", "LOCALIZE[30200]", config.AddonIcon())
}
