package api

import (
	"fmt"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/tmdb"
	"github.com/steeve/pulsar/tvdb"
	"github.com/steeve/pulsar/xbmc"
)

func renderShows(shows tmdb.Shows, ctx *gin.Context) {
	items := make(xbmc.ListItems, 0, len(shows))
	for _, show := range shows {
		item := show.ToListItem()
		item.Path = UrlForXBMC("/show/%d/seasons", show.ExternalIDs.TVDBID)
		items = append(items, item)
	}

	ctx.JSON(200, xbmc.NewView("tvshows", items))
}

func PopularShows(ctx *gin.Context) {
	renderShows(tmdb.PopularShowsComplete(""), ctx)
}

func TopRatedShows(ctx *gin.Context) {
	renderShows(tmdb.TopRatedShowsComplete(""), ctx)
}

func SearchShows(ctx *gin.Context) {
	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		query = xbmc.Keyboard("", "Search TV Shows")
	}
	renderShows(tmdb.SearchShows(query, "en"), ctx)
}

func ShowSeasons(ctx *gin.Context) {
	show, err := tvdb.NewShow(ctx.Params.ByName("showId"), "en")
	if err != nil {
		ctx.Error(err, nil)
		return
	}

	items := show.Seasons.ToListItems()
	reversedItems := make(xbmc.ListItems, 0)
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		item.Path = UrlForXBMC("/show/%d/season/%d/episodes", show.Id, item.Info.Season)
		reversedItems = append(reversedItems, item)
	}
	// xbmc.ListItems always returns false to Less() so that order is unchanged

	ctx.JSON(200, xbmc.NewView("seasons", reversedItems))
}

func ShowEpisodes(ctx *gin.Context) {
	show, err := tvdb.NewShow(ctx.Params.ByName("showId"), "en")
	if err != nil {
		ctx.Error(err, nil)
		return
	}

	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))

	season := show.Seasons[seasonNumber]
	items := season.Episodes.ToListItems()
	for _, item := range items {
		item.Path = UrlForXBMC("/show/%d/season/%d/episode/%d/play",
			show.Id,
			season.Season,
			item.Info.Episode,
		)
		item.ContextMenu = [][]string{
			[]string{"Choose stream...", fmt.Sprintf("XBMC.PlayMedia(%s)", UrlForXBMC("/show/%d/season/%d/episode/%d/links",
				show.Id,
				season.Season,
				item.Info.Episode,
			))},
		}
		item.IsPlayable = true
	}

	ctx.JSON(200, xbmc.NewView("episodes", items))
}

func showEpisodeLinks(showId string, seasonNumber, episodeNumber int) ([]*bittorrent.Torrent, error) {
	log.Println("Searching links for TVDB Id:", showId)

	show, err := tvdb.NewShow(showId, "en")
	if err != nil {
		return nil, err
	}

	episode := show.Seasons[seasonNumber].Episodes[episodeNumber-1]

	log.Printf("Resolved %s to %s\n", showId, show.SeriesName)

	searchers := providers.GetEpisodeSearchers()

	return providers.SearchEpisode(searchers, episode), nil
}

func ShowEpisodeLinks(ctx *gin.Context) {
	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))
	episodeNumber, _ := strconv.Atoi(ctx.Params.ByName("episode"))
	torrents, err := showEpisodeLinks(ctx.Params.ByName("showId"), seasonNumber, episodeNumber)
	if err != nil {
		ctx.Error(err, nil)
		return
	}

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
	torrents, err := showEpisodeLinks(ctx.Params.ByName("showId"), seasonNumber, episodeNumber)
	if err != nil {
		ctx.Error(err, nil)
		return
	}

	if len(torrents) == 0 {
		xbmc.Notify("Pulsar", "No links were found.")
		return
	}

	rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[0].Magnet())
	ctx.Redirect(302, rUrl)
}
