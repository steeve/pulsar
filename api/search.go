package api

import (
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/xbmc"
)

func Search(c *gin.Context) {
	query := xbmc.Keyboard("", "Search")
	if query == "" {
		return
	}

	log.Println("Searching providers for:", query)

	searchers := make([]providers.Searcher, 0)
	for _, addon := range xbmc.GetAddons("xbmc.python.script").Addons {
		if strings.HasPrefix(addon.ID, "script.pulsar.") {
			searchers = append(searchers, providers.NewAddonSearcher(addon.ID))
		}
	}

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
