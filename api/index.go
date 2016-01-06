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
		{Label: "Movies", Path: UrlForXBMC("/movies/"), Thumbnail: config.AddonResource("img", "movies.png")},
		{Label: "TV Shows", Path: UrlForXBMC("/shows/"), Thumbnail: config.AddonResource("img", "tv.png")},

		{Label: "Search", Path: UrlForXBMC("/search"), Thumbnail: config.AddonResource("img", "search.png")},
		{Label: "Paste URL", Path: UrlForXBMC("/pasted"), Thumbnail: config.AddonResource("img", "magnet.png")},
	}))
}
