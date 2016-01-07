package api

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/i96751414/pulsar/providers"
	"github.com/i96751414/pulsar/xbmc"
)

func Search(c *gin.Context) {
	query := xbmc.Keyboard("", xbmc.GetLocalizedString(32009))
	if query == "" {
		return
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

	c.JSON(200, xbmc.NewView("", items))
}
