package api

import (
	"fmt"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/trakt"
	"github.com/steeve/pulsar/xbmc"
)

func renderShows(shows trakt.ShowList, ctx *gin.Context) {
	items := make(xbmc.ListItems, 0, len(shows))
	for _, show := range shows {
		item := show.ToListItem()
		item.Path = UrlForXBMC("/show/%s/seasons", show.TVDBId)
		items = append(items, item)
	}

	ctx.JSON(200, xbmc.NewView("tvshows", items))
}

func PopularShows(ctx *gin.Context) {
	renderShows(trakt.TrendingShows(), ctx)
}

func SearchShows(ctx *gin.Context) {
	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		query = xbmc.Keyboard("", "Search Movies")
	}
	renderShows(trakt.SearchShows(query), ctx)
}

func ShowSeasons(ctx *gin.Context) {
	show := trakt.NewShow(ctx.Params.ByName("showId"))
	seasons := show.Seasons()

	items := make(xbmc.ListItems, 0, len(seasons))
	for _, season := range seasons {
		if season.Season == 0 {
			continue
		}
		item := season.ToListItem()
		item.Path = UrlForXBMC("/show/%s/season/%d/episodes", show.TVDBId, season.Season)
		items = append(items, item)
	}

	ctx.JSON(200, xbmc.NewView("seasons", items))
}

func ShowEpisodes(ctx *gin.Context) {
	show := trakt.NewShow(ctx.Params.ByName("showId"))
	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))
	season := show.Season(seasonNumber)
	episodes := season.Episodes()

	items := make(xbmc.ListItems, 0, len(episodes))
	for _, episode := range episodes {
		item := episode.ToListItem()
		item.Path = UrlForXBMC("/show/%s/season/%d/episode/%d/play",
			show.TVDBId,
			season.Season,
			episode.Episode,
		)
		item.ContextMenu = [][]string{
			[]string{"Choose stream...", fmt.Sprintf("XBMC.PlayMedia(%s)", UrlForXBMC("/show/%s/season/%d/episode/%d/links",
				show.TVDBId,
				season.Season,
				episode.Episode,
			))},
		}
		item.IsPlayable = true
		items = append(items, item)
	}

	ctx.JSON(200, xbmc.NewView("episodes", items))
}

func showEpisodeLinks(showId string, seasonNumber, episodeNumber int) []*bittorrent.Torrent {
	log.Println("Searching links for TVDB Id:", showId)

	show := trakt.NewShow(showId)
	episode := show.Season(seasonNumber).Episode(episodeNumber)

	log.Printf("Resolved %s to %s\n", showId, show.Title)

	searchers := providers.GetEpisodeSearchers()

	return providers.SearchEpisode(searchers, episode)
}

func ShowEpisodeLinks(ctx *gin.Context) {
	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))
	episodeNumber, _ := strconv.Atoi(ctx.Params.ByName("episode"))
	torrents := showEpisodeLinks(ctx.Params.ByName("showId"), seasonNumber, episodeNumber)

	if len(torrents) == 0 {
		xbmc.Notify("Pulsar", "No links were found.")
		return
	}

	choices := make([]string, 0, len(torrents))
	for _, torrent := range torrents {
		label := fmt.Sprintf("S:%d P:%d - %s",
			torrent.Seeds,
			torrent.Peers,
			torrent.Name,
		)
		choices = append(choices, label)
	}

	choice := xbmc.ListDialog("Choose stream", choices...)
	if choice >= 0 {
		rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[choice].Magnet())
		ctx.Redirect(302, rUrl)
	}
}

func ShowEpisodePlay(ctx *gin.Context) {
	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))
	episodeNumber, _ := strconv.Atoi(ctx.Params.ByName("episode"))
	torrents := showEpisodeLinks(ctx.Params.ByName("showId"), seasonNumber, episodeNumber)

	if len(torrents) == 0 {
		xbmc.Notify("Pulsar", "No links were found.")
		return
	}

	rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[0].Magnet())
	ctx.Redirect(302, rUrl)
}
