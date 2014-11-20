package api

import (
	"github.com/gin-gonic/gin"
	"github.com/steeve/pulsar/xbmc"
)

func Index(ctx *gin.Context) {
	action := ctx.Request.URL.Query().Get("action")
	if action == "search" || action == "manualsearch" {
		SubtitlesIndex(ctx)
		return
	}

	ctx.JSON(200, xbmc.NewView("", xbmc.ListItems{
		{Label: "Movies", Path: UrlForXBMC("/movies/")},
		{Label: "TV Shows", Path: UrlForXBMC("/shows/")},

		{Label: "Search", Path: UrlForXBMC("/search")},
		{Label: "Paste URL", Path: UrlForXBMC("/pasted")},
	}))
}
