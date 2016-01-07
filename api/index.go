package api

import (
	"github.com/gin-gonic/gin"
	"github.com/i96751414/pulsar/config"
	"github.com/i96751414/pulsar/xbmc"
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
		{Label: xbmc.GetLocalizedString(32014), Path: UrlForXBMC("/movies/"), Thumbnail: config.AddonResource("img", "movies.png")},
		{Label: xbmc.GetLocalizedString(32015), Path: UrlForXBMC("/shows/"), Thumbnail: config.AddonResource("img", "tv.png")},

		{Label: xbmc.GetLocalizedString(32009), Path: UrlForXBMC("/search"), Thumbnail: config.AddonResource("img", "search.png")},
		{Label: xbmc.GetLocalizedString(32016), Path: UrlForXBMC("/pasted"), Thumbnail: config.AddonResource("img", "magnet.png")},
	}))
}
