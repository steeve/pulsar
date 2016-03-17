package api

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/providers"
	"github.com/scakemyer/quasar/xbmc"
)

var searchHistory []string

func Search(ctx *gin.Context) {
	query := ctx.Request.URL.Query().Get("q")
	if len(searchHistory) > 0 && xbmc.DialogConfirm("Quasar", "LOCALIZE[30262]") {
		choice := xbmc.ListDialog("LOCALIZE[30261]", searchHistory...)
		query = searchHistory[choice]
	} else {
		query = xbmc.Keyboard("", "LOCALIZE[30209]")
		if query == "" {
			return
		}
		searchHistory = append(searchHistory, query)
	}

	log.Println("Searching providers for:", query)

	searchers := providers.GetSearchers()
	torrents := providers.Search(searchers, query)

	items := make(xbmc.ListItems, 0, len(torrents))
	for _, torrent := range torrents {
		item := &xbmc.ListItem{
			Label:      fmt.Sprintf("S:%d P:%d - %s", torrent.Seeds, torrent.Peers, torrent.Name),
			Path:       UrlQuery(UrlForXBMC("/play"), "uri", torrent.URI),
			IsPlayable: true,
		}
		items = append(items, item)
	}

	ctx.JSON(200, xbmc.NewView("", items))
}
