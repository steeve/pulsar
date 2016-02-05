package api

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
	"github.com/scakemyer/quasar/bittorrent"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/providers"
	"github.com/scakemyer/quasar/tmdb"
	"github.com/scakemyer/quasar/tvdb"
	"github.com/scakemyer/quasar/xbmc"
)

var (
	showsLog = logging.MustGetLogger("shows")
)

func TVIndex(ctx *gin.Context) {
	items := xbmc.ListItems{
		{Label: "LOCALIZE[30209]", Path: UrlForXBMC("/shows/search"), Thumbnail: config.AddonResource("img", "search.png")},
		{Label: "LOCALIZE[30210]", Path: UrlForXBMC("/shows/popular"), Thumbnail: config.AddonResource("img", "popular.png")},
	}
	for _, genre := range tmdb.GetTVGenres(config.Get().Language) {
		slug, _ := genreSlugs[genre.Id]
		items = append(items, &xbmc.ListItem{
			Label:     genre.Name,
			Path:      UrlForXBMC("/shows/popular/%s", strconv.Itoa(genre.Id)),
			Thumbnail: config.AddonResource("img", fmt.Sprintf("genre_%s.png", slug)),
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

func renderShows(shows tmdb.Shows, ctx *gin.Context, page int) {
	paging := 0
	if page >= 0 {
		paging = 1
	}
	items := make(xbmc.ListItems, 0, len(shows) + paging)
	for _, show := range shows {
		if show == nil {
			continue
		}
		item := show.ToListItem()
		item.Path = UrlForXBMC("/show/%d/seasons", show.ExternalIDs.TVDBID)
		item.ContextMenu = [][]string{
			[]string{"LOCALIZE[30219]", fmt.Sprintf("XBMC.PlayMedia(%s)", UrlForXBMC("/library/show/addremove/%d", show.ExternalIDs.TVDBID))},
		}
		items = append(items, item)
	}
	if page >= 0 {
		path := ctx.Request.URL.Path
		nextpage := &xbmc.ListItem{Label: "LOCALIZE[30218]", Path: UrlForXBMC(fmt.Sprintf("%s?page=%d", path, page + 1)), Thumbnail: config.AddonResource("img", "nextpage.png")}
		items = append(items, nextpage)
	}
	ctx.JSON(200, xbmc.NewView("tvshows", items))
}

func PopularShows(ctx *gin.Context) {
	genre := ctx.Params.ByName("genre")
	if genre == "0" {
		genre = ""
	}
	page := -1
	if config.Get().EnablePaging == true {
		currentpage, err := strconv.Atoi(ctx.DefaultQuery("page", "0"))
    	if err == nil {
			page = currentpage
		}
	}
	renderShows(tmdb.PopularShowsComplete(genre, config.Get().Language, page), ctx, page)
}

func TopRatedShows(ctx *gin.Context) {
	page := -1
	if config.Get().EnablePaging == true {
		currentpage, err := strconv.Atoi(ctx.DefaultQuery("page", "0"))
    	if err == nil {
			page = currentpage
		}
	}
	renderShows(tmdb.TopRatedShowsComplete("", config.Get().Language, page), ctx, page)
}

func TVMostVoted(ctx *gin.Context) {
	page := -1
	if config.Get().EnablePaging == true {
		currentpage, err := strconv.Atoi(ctx.DefaultQuery("page", "0"))
    	if err == nil {
			page = currentpage
		}
	}
	renderMovies(tmdb.MostVotedShowsComplete("", config.Get().Language, page), ctx, page)
}

func SearchShows(ctx *gin.Context) {
	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		query = xbmc.Keyboard("", "LOCALIZE[30201]")
		if query == "" {
			return
		}
	}
	renderShows(tmdb.SearchShows(query, config.Get().Language), ctx, -1)
}

func ShowSeasons(ctx *gin.Context) {
	show, err := tvdb.NewShowCached(ctx.Params.ByName("showId"), config.Get().Language)
	if err != nil {
		ctx.Error(err)
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
		ctx.Error(err)
		return
	}

	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))

	season := show.Seasons[seasonNumber]
	items := season.Episodes.ToListItems(show)
	for _, item := range items {
		playUrl := UrlForXBMC("/show/%d/season/%d/episode/%d/play",
			show.Id,
			season.Season,
			item.Info.Episode,
		)
		episodeLinksUrl := UrlForXBMC("/show/%d/season/%d/episode/%d/links",
			show.Id,
			season.Season,
			item.Info.Episode,
		)
		if config.Get().ChooseStreamAuto == true {
			item.Path = playUrl
		} else {
			item.Path = episodeLinksUrl
		}
		item.ContextMenu = [][]string{
			[]string{"LOCALIZE[30202]", fmt.Sprintf("XBMC.PlayMedia(%s)", episodeLinksUrl)},
			[]string{"LOCALIZE[30023]", fmt.Sprintf("XBMC.PlayMedia(%s)", playUrl)},
			[]string{"LOCALIZE[30203]", "XBMC.Action(Info)"},
		}
		item.IsPlayable = true
	}

	ctx.JSON(200, xbmc.NewView("episodes", items))
}

func showEpisodeLinks(showId string, seasonNumber, episodeNumber int) ([]*bittorrent.Torrent, string, error) {
	log.Println("Searching links for TVDB Id:", showId)

	show, err := tvdb.NewShowCached(showId, config.Get().Language)
	if err != nil {
		return nil, "", err
	}

	episode := show.Seasons[seasonNumber].Episodes[episodeNumber-1]

	log.Printf("Resolved %s to %s\n", showId, show.SeriesName)

	searchers := providers.GetEpisodeSearchers()
	if len(searchers) == 0 {
		xbmc.Notify("Quasar", "LOCALIZE[30204]", config.AddonIcon())
	}

	longName := fmt.Sprintf("%s S%02dE%02d", show.SeriesName, seasonNumber, episodeNumber)

	return providers.SearchEpisode(searchers, show, episode), longName, nil
}

func ShowEpisodeLinks(ctx *gin.Context) {
	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))
	episodeNumber, _ := strconv.Atoi(ctx.Params.ByName("episode"))
	torrents, longName, err := showEpisodeLinks(ctx.Params.ByName("showId"), seasonNumber, episodeNumber)
	if err != nil {
		ctx.Error(err)
		return
	}

	if len(torrents) == 0 {
		xbmc.Notify("Quasar", "LOCALIZE[30205]", config.AddonIcon())
		return
	}

	choices := make([]string, 0, len(torrents))
	for _, torrent := range torrents {
		info := make([]string, 0)
		if torrent.Resolution > 0 {
			info = append(info, bittorrent.Resolutions[torrent.Resolution])
		}
		if torrent.Size != "" {
			info = append(info, fmt.Sprintf("[%s]", torrent.Size))
		}
		if torrent.RipType > 0 {
			info = append(info, bittorrent.Rips[torrent.RipType])
		}
		if torrent.VideoCodec > 0 {
			info = append(info, bittorrent.Codecs[torrent.VideoCodec])
		}
		if torrent.AudioCodec > 0 {
			info = append(info, bittorrent.Codecs[torrent.AudioCodec])
		}
		if torrent.Provider != "" {
			info = append(info, " - " + torrent.Provider)
		}

		torrentName := torrent.Name
		if len(torrentName) > 80 {
			torrentName = torrentName[:80]
		}

		label := fmt.Sprintf("[B](%d / %d) %s[/B]\n%s",
			torrent.Seeds,
			torrent.Peers,
			strings.Join(info, " "),
			torrentName,
		)
		choices = append(choices, label)
	}

	choice := xbmc.ListDialogLarge("LOCALIZE[30228]", longName, choices...)
	if choice >= 0 {
		rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[choice].Magnet())
		ctx.Redirect(302, rUrl)
	}
}

func ShowEpisodePlay(ctx *gin.Context) {
	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))
	episodeNumber, _ := strconv.Atoi(ctx.Params.ByName("episode"))
	torrents, _, err := showEpisodeLinks(ctx.Params.ByName("showId"), seasonNumber, episodeNumber)
	if err != nil {
		ctx.Error(err)
		return
	}

	if len(torrents) == 0 {
		xbmc.Notify("Quasar", "LOCALIZE[30205]", config.AddonIcon())
		return
	}

	rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[0].Magnet())
	ctx.Redirect(302, rUrl)
}
