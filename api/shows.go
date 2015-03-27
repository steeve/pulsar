package api

import (
	"fmt"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/config"
	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/tmdb"
	"github.com/steeve/pulsar/tvdb"
	"github.com/steeve/pulsar/xbmc"
)

var (
	showsLog = logging.MustGetLogger("shows")
)

func TVIndex(ctx *gin.Context) {
	items := xbmc.ListItems{
		{Label: "Search", Path: UrlForXBMC("/shows/search"), Thumbnail: AddonResource("img", "search.png")},
		{Label: "Most Popular", Path: UrlForXBMC("/shows/popular"), Thumbnail: AddonResource("img", "popular.png")},
	}
	for _, genre := range tmdb.GetTVGenres(config.Get().Language) {
		slug, _ := genreSlugs[genre.Id]
		items = append(items, &xbmc.ListItem{
			Label:     genre.Name,
			Path:      UrlForXBMC("/shows/popular/%s", strconv.Itoa(genre.Id)),
			Thumbnail: AddonResource("img", fmt.Sprintf("genre_%s.png", slug)),
		})
	}

	ctx.JSON(200, xbmc.NewView("", items))
}

func TVGenres(ctx *gin.Context) {
	genres := tmdb.GetTVGenres(config.Get().Language)
	items := make(xbmc.ListItems, 0, len(genres))
	for _, genre := range genres {
		items = append(items, &xbmc.ListItem{
			Label: genre.Name,
			Path:  UrlForXBMC("/shows/popular/%s", strconv.Itoa(genre.Id)),
		})
	}

	ctx.JSON(200, xbmc.NewView("", items))
}

func renderShows(shows tmdb.Shows, ctx *gin.Context) {
	items := make(xbmc.ListItems, 0, len(shows))
	for _, show := range shows {
		if show == nil {
			continue
		}
		item := show.ToListItem()
		item.Path = UrlForXBMC("/show/%d/seasons", show.ExternalIDs.TVDBID)
		items = append(items, item)
	}

	ctx.JSON(200, xbmc.NewView("tvshows", items))
}

func PopularShows(ctx *gin.Context) {
	genre := ctx.Params.ByName("genre")
	if genre == "0" {
		genre = ""
	}
	renderShows(tmdb.PopularShowsComplete(genre, config.Get().Language), ctx)
}

func TopRatedShows(ctx *gin.Context) {
	renderShows(tmdb.TopRatedShowsComplete("", config.Get().Language), ctx)
}

func TVMostVoted(ctx *gin.Context) {
	renderMovies(tmdb.MostVotedShowsComplete("", config.Get().Language), ctx)
}

func SearchShows(ctx *gin.Context) {
	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		query = xbmc.Keyboard("", "Search TV Shows")
	}
	renderShows(tmdb.SearchShows(query, config.Get().Language), ctx)
}

func ShowSeasons(ctx *gin.Context) {
	show, err := tvdb.NewShowCached(ctx.Params.ByName("showId"), config.Get().Language)
	if err != nil {
		ctx.Error(err, nil)
		return
	}

	items := show.Seasons.ToListItems(show)
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
	show, err := tvdb.NewShowCached(ctx.Params.ByName("showId"), config.Get().Language)
	if err != nil {
		ctx.Error(err, nil)
		return
	}

	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))

	season := show.Seasons[seasonNumber]
	items := season.Episodes.ToListItems(show)
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

	show, err := tvdb.NewShowCached(showId, config.Get().Language)
	if err != nil {
		return nil, err
	}

	episode := show.Seasons[seasonNumber].Episodes[episodeNumber-1]

	log.Printf("Resolved %s to %s\n", showId, show.SeriesName)

	searchers := providers.GetEpisodeSearchers()
	if len(searchers) == 0 {
		xbmc.Notify("Pulsar", "Unable to find any providers")
	}

	return providers.SearchEpisode(searchers, show, episode), nil
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
		xbmc.Notify("Pulsar", "No links were found")
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
		xbmc.Notify("Pulsar", "No links were found")
		return
	}

	rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[0].Magnet())
	ctx.Redirect(302, rUrl)
}
