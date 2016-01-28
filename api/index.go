package api

import (
	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/xbmc"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

func Index(ctx *gin.Context) {
	action := ctx.Request.URL.Query().Get("action")
	if action == "search" || action == "manualsearch" {
		SubtitlesIndex(ctx)
		return
	}

	ctx.JSON(200, xbmc.NewView("", xbmc.ListItems{
		{Label: "LOCALIZE[30214]", Path: UrlForXBMC("/movies/"), Thumbnail: config.AddonResource("img", "movies.png")},
		{Label: "LOCALIZE[30215]", Path: UrlForXBMC("/shows/"), Thumbnail: config.AddonResource("img", "tv.png")},

		{Label: "LOCALIZE[30209]", Path: UrlForXBMC("/search"), Thumbnail: config.AddonResource("img", "search.png")},
		{Label: "LOCALIZE[30216]", Path: UrlForXBMC("/pasted"), Thumbnail: config.AddonResource("img", "magnet.png")},
	}))
}
