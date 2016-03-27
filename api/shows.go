package api

import (
	"fmt"
	"log"
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/bittorrent"
	"github.com/scakemyer/quasar/providers"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/tmdb"
	"github.com/scakemyer/quasar/xbmc"
)


func TVIndex(ctx *gin.Context) {
	items := xbmc.ListItems{
		{Label: "LOCALIZE[30209]", Path: UrlForXBMC("/shows/search"), Thumbnail: config.AddonResource("img", "search.png")},
		{Label: "LOCALIZE[30056]", Path: UrlForXBMC("/shows/trakt/"), Thumbnail: config.AddonResource("img", "trakt.png")},
		{Label: "LOCALIZE[30238]", Path: UrlForXBMC("/shows/recent/episodes"), Thumbnail: config.AddonResource("img", "fresh.png")},
		{Label: "LOCALIZE[30237]", Path: UrlForXBMC("/shows/recent/shows"), Thumbnail: config.AddonResource("img", "clock.png")},
		{Label: "LOCALIZE[30210]", Path: UrlForXBMC("/shows/popular"), Thumbnail: config.AddonResource("img", "popular.png")},
		{Label: "LOCALIZE[30211]", Path: UrlForXBMC("/shows/top"), Thumbnail: config.AddonResource("img", "top_rated.png")},
		{Label: "LOCALIZE[30212]", Path: UrlForXBMC("/shows/mostvoted"), Thumbnail: config.AddonResource("img", "most_voted.png")},
	}
	for _, genre := range tmdb.GetTVGenres(config.Get().Language) {
		slug, _ := genreSlugs[genre.Id]
		items = append(items, &xbmc.ListItem{
			Label:     genre.Name,
			Path:      UrlForXBMC("/shows/popular/%s", strconv.Itoa(genre.Id)),
			Thumbnail: config.AddonResource("img", fmt.Sprintf("genre_%s.png", slug)),
			ContextMenu: [][]string{
				[]string{"LOCALIZE[30237]", fmt.Sprintf("Container.Update(%s)", UrlForXBMC("/shows/recent/shows/%s", strconv.Itoa(genre.Id)))},
				[]string{"LOCALIZE[30238]", fmt.Sprintf("Container.Update(%s)", UrlForXBMC("/shows/recent/episodes/%s", strconv.Itoa(genre.Id)))},
			},
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

func TVTrakt(ctx *gin.Context) {
	items := xbmc.ListItems{
		{Label: "LOCALIZE[30254]", Path: UrlForXBMC("/shows/trakt/watchlist"), Thumbnail: config.AddonResource("img", "trakt.png")},
		{Label: "LOCALIZE[30257]", Path: UrlForXBMC("/shows/trakt/collection"), Thumbnail: config.AddonResource("img", "trakt.png")},
		{Label: "LOCALIZE[30210]", Path: UrlForXBMC("/shows/trakt/popular"), Thumbnail: config.AddonResource("img", "popular.png")},
		{Label: "LOCALIZE[30246]", Path: UrlForXBMC("/shows/trakt/trending"), Thumbnail: config.AddonResource("img", "trending.png")},
		{Label: "LOCALIZE[30247]", Path: UrlForXBMC("/shows/trakt/played"), Thumbnail: config.AddonResource("img", "most_played.png")},
		{Label: "LOCALIZE[30248]", Path: UrlForXBMC("/shows/trakt/watched"), Thumbnail: config.AddonResource("img", "most_watched.png")},
		{Label: "LOCALIZE[30249]", Path: UrlForXBMC("/shows/trakt/collected"), Thumbnail: config.AddonResource("img", "most_collected.png")},
		{Label: "LOCALIZE[30250]", Path: UrlForXBMC("/shows/trakt/anticipated"), Thumbnail: config.AddonResource("img", "most_anticipated.png")},
	}
	ctx.JSON(200, xbmc.NewView("", items))
}

func renderShows(shows tmdb.Shows, ctx *gin.Context, page int, query string) {
	nextPage := 0
	if page >= 0 {
		nextPage = 1
	}
	items := make(xbmc.ListItems, 0, len(shows) + nextPage)
	for _, show := range shows {
		if show == nil {
			continue
		}
		item := show.ToListItem()
		item.Path = UrlForXBMC("/show/%d/seasons", show.Id)

		libraryAction := []string{"LOCALIZE[30252]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/library/show/add/%d", show.Id))}
		if inJsonDb, err := InJsonDB(strconv.Itoa(show.Id), LShow); err == nil && inJsonDb == true {
			libraryAction = []string{"LOCALIZE[30253]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/library/show/remove/%d", show.Id))}
		}

		watchlistAction := []string{"LOCALIZE[30255]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/show/%d/watchlist/add", show.Id))}
		if InShowsWatchlist(show.Id) {
			watchlistAction = []string{"LOCALIZE[30256]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/show/%d/watchlist/remove", show.Id))}
		}

		collectionAction := []string{"LOCALIZE[30258]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/show/%d/collection/add", show.Id))}
		if InShowsCollection(show.Id) {
			collectionAction = []string{"LOCALIZE[30259]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/show/%d/collection/remove", show.Id))}
		}

		item.ContextMenu = [][]string{
			libraryAction,
			watchlistAction,
			collectionAction,
			[]string{"LOCALIZE[30035]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/setviewmode/tvshows"))},
		}
		items = append(items, item)
	}
	if page >= 0 {
		path := ctx.Request.URL.Path
		nextPath := UrlForXBMC(fmt.Sprintf("%s?page=%d", path, page + 1))
		if query != "" {
			nextPath = UrlForXBMC(fmt.Sprintf("%s?q=%s&page=%d", path, query, page + 1))
		}
		next := &xbmc.ListItem{
			Label: "LOCALIZE[30218]",
			Path: nextPath,
			Thumbnail: config.AddonResource("img", "nextpage.png"),
		}
		items = append(items, next)
	}
	ctx.JSON(200, xbmc.NewView("tvshows", items))
}

func PopularShows(ctx *gin.Context) {
	genre := ctx.Params.ByName("genre")
	if genre == "0" {
		genre = ""
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	renderShows(tmdb.PopularShows(genre, config.Get().Language, page), ctx, page, "")
}

func RecentShows(ctx *gin.Context) {
	genre := ctx.Params.ByName("genre")
	if genre == "0" {
		genre = ""
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	renderShows(tmdb.RecentShows(genre, config.Get().Language, page), ctx, page, "")
}

func RecentEpisodes(ctx *gin.Context) {
	genre := ctx.Params.ByName("genre")
	if genre == "0" {
		genre = ""
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	renderShows(tmdb.RecentEpisodes(genre, config.Get().Language, page), ctx, page, "")
}

func TopRatedShows(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	renderShows(tmdb.TopRatedShows("", config.Get().Language, page), ctx, page, "")
}

func TVMostVoted(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	renderShows(tmdb.MostVotedShows("", config.Get().Language, page), ctx, page, "")
}

func SearchShows(ctx *gin.Context) {
	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		if len(searchHistory) > 0 && xbmc.DialogConfirm("Quasar", "LOCALIZE[30262]") {
			choice := xbmc.ListDialog("LOCALIZE[30261]", searchHistory...)
			query = searchHistory[choice]
		} else {
			query = xbmc.Keyboard("", "LOCALIZE[30201]")
			if query == "" {
				return
			}
			searchHistory = append(searchHistory, query)
		}
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	renderShows(tmdb.SearchShows(query, config.Get().Language, page), ctx, page, query)
}

func ShowSeasons(ctx *gin.Context) {
	showId, _ := strconv.Atoi(ctx.Params.ByName("showId"))

	show := tmdb.GetShow(showId, config.Get().Language)

	items := show.Seasons.ToListItems(show)
	reversedItems := make(xbmc.ListItems, 0)
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		item.Path = UrlForXBMC("/show/%d/season/%d/episodes", show.Id, item.Info.Season)
		item.ContextMenu = [][]string{
			[]string{"LOCALIZE[30202]", fmt.Sprintf("XBMC.PlayMedia(%s)", UrlForXBMC("/show/%d/season/%d/links", show.Id, item.Info.Season))},
			[]string{"LOCALIZE[30036]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/setviewmode/seasons"))},
		}
		reversedItems = append(reversedItems, item)
	}
	// xbmc.ListItems always returns false to Less() so that order is unchanged

	ctx.JSON(200, xbmc.NewView("seasons", reversedItems))
}

func ShowEpisodes(ctx *gin.Context) {
	showId, _ := strconv.Atoi(ctx.Params.ByName("showId"))
	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))
	language := config.Get().Language
	show := tmdb.GetShow(showId, language)
	season := tmdb.GetSeason(showId, seasonNumber, language)
	items := season.Episodes.ToListItems(show, season)

	for _, item := range items {
		playUrl := UrlForXBMC("/show/%d/season/%d/episode/%d/play",
			show.Id,
			seasonNumber,
			item.Info.Episode,
		)
		episodeLinksUrl := UrlForXBMC("/show/%d/season/%d/episode/%d/links",
			show.Id,
			seasonNumber,
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
			[]string{"LOCALIZE[30037]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/setviewmode/episodes"))},
		}
		item.IsPlayable = true
	}

	ctx.JSON(200, xbmc.NewView("episodes", items))
}

func showSeasonLinks(showId int, seasonNumber int) ([]*bittorrent.Torrent, error) {
	log.Println("Searching links for TMDB Id:", showId)

	show := tmdb.GetShow(showId, config.Get().Language)
	season := tmdb.GetSeason(showId, seasonNumber, config.Get().Language)
	if season == nil {
		return nil, errors.New("Unable to find season")
	}

	log.Printf("Resolved %d to %s", showId, show.Name)

	if torrents := InTorrentsMap(strconv.Itoa(season.Id)); len(torrents) > 0 {
		return torrents, nil
	}

	searchers := providers.GetSeasonSearchers()
	if len(searchers) == 0 {
		xbmc.Notify("Quasar", "LOCALIZE[30204]", config.AddonIcon())
	}

	return providers.SearchSeason(searchers, show, season), nil
}

func ShowSeasonLinks(ctx *gin.Context) {
	showId, _ := strconv.Atoi(ctx.Params.ByName("showId"))
	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))

	show := tmdb.GetShow(showId, "")
	season := tmdb.GetSeason(showId, seasonNumber,"")
	longName := fmt.Sprintf("%s Season %02d", show.Name, seasonNumber)

	torrents, err := showSeasonLinks(showId, seasonNumber)
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
		resolution := ""
		if torrent.Resolution > 0 {
			resolution = fmt.Sprintf("[B][COLOR %s]%s[/COLOR][/B] ", bittorrent.Colors[torrent.Resolution], bittorrent.Resolutions[torrent.Resolution])
		}

		info := make([]string, 0)
		if torrent.Size != "" {
			info = append(info, fmt.Sprintf("[B][%s][/B]", torrent.Size))
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
			info = append(info, fmt.Sprintf(" - [B]%s[/B]", torrent.Provider))
		}

		multi := ""
		if torrent.Multi {
			multi = "\nmulti"
		}

		label := fmt.Sprintf("%s(%d / %d) %s\n%s\n%s%s",
			resolution,
			torrent.Seeds,
			torrent.Peers,
			strings.Join(info, " "),
			torrent.Name,
			torrent.Icon,
			multi,
		)
		choices = append(choices, label)
	}

	choice := xbmc.ListDialogLarge("LOCALIZE[30228]", longName, choices...)
	if choice >= 0 {
		AddToTorrentsMap(strconv.Itoa(season.Id), torrents[choice])

		rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[choice].Magnet())
		ctx.Redirect(302, rUrl)
	}
}

func showEpisodeLinks(showId int, seasonNumber int, episodeNumber int) ([]*bittorrent.Torrent, error) {
	log.Println("Searching links for TMDB Id:", showId)

	show := tmdb.GetShow(showId, config.Get().Language)
	season := tmdb.GetSeason(showId, seasonNumber, config.Get().Language)
	if season == nil {
		return nil, errors.New("Unable to find season")
	}

	episode := season.Episodes[episodeNumber - 1]

	log.Printf("Resolved %d to %s", showId, show.Name)

	if torrents := InTorrentsMap(strconv.Itoa(episode.Id)); len(torrents) > 0 {
		return torrents, nil
	}

	searchers := providers.GetEpisodeSearchers()
	if len(searchers) == 0 {
		xbmc.Notify("Quasar", "LOCALIZE[30204]", config.AddonIcon())
	}

	return providers.SearchEpisode(searchers, show, episode), nil
}

func ShowEpisodeLinks(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("showId")
	showId, _ := strconv.Atoi(tmdbId)
	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))
	episodeNumber, _ := strconv.Atoi(ctx.Params.ByName("episode"))

	show := tmdb.GetShow(showId, "")
	episode := tmdb.GetEpisode(showId, seasonNumber, episodeNumber, "")
	longName := fmt.Sprintf("%s S%02dE%02d", show.Name, seasonNumber, episodeNumber)

	runtime := 45
	if len(show.EpisodeRunTime) > 0 {
		runtime = show.EpisodeRunTime[len(show.EpisodeRunTime) - 1]
	}

	torrents, err := showEpisodeLinks(showId, seasonNumber, episodeNumber)
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
		resolution := ""
		if torrent.Resolution > 0 {
			resolution = fmt.Sprintf("[B][COLOR %s]%s[/COLOR][/B] ", bittorrent.Colors[torrent.Resolution], bittorrent.Resolutions[torrent.Resolution])
		}

		info := make([]string, 0)
		if torrent.Size != "" {
			info = append(info, fmt.Sprintf("[B][%s][/B]", torrent.Size))
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
			info = append(info, fmt.Sprintf(" - [B]%s[/B]", torrent.Provider))
		}

		multi := ""
		if torrent.Multi {
			multi = "\nmulti"
		}

		label := fmt.Sprintf("%s(%d / %d) %s\n%s\n%s%s",
			resolution,
			torrent.Seeds,
			torrent.Peers,
			strings.Join(info, " "),
			torrent.Name,
			torrent.Icon,
			multi,
		)
		choices = append(choices, label)
	}

	choice := xbmc.ListDialogLarge("LOCALIZE[30228]", longName, choices...)
	if choice >= 0 {
		AddToTorrentsMap(strconv.Itoa(episode.Id), torrents[choice])

		rUrl := UrlQuery(
			UrlForXBMC("/play"), "uri", torrents[choice].Magnet(),
			                     "tmdb", strconv.Itoa(episode.Id),
			                     "type", "episode",
			                     "runtime", strconv.Itoa(runtime))
		ctx.Redirect(302, rUrl)
	}
}

func ShowEpisodePlay(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("showId")
	showId, _ := strconv.Atoi(tmdbId)
	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))
	episodeNumber, _ := strconv.Atoi(ctx.Params.ByName("episode"))

	show := tmdb.GetShow(showId, "")
	episode := tmdb.GetEpisode(showId, seasonNumber, episodeNumber, "")

	runtime := 45
	if len(show.EpisodeRunTime) > 0 {
		runtime = show.EpisodeRunTime[len(show.EpisodeRunTime) - 1]
	}

	torrents, err := showEpisodeLinks(showId, seasonNumber, episodeNumber)
	if err != nil {
		ctx.Error(err)
		return
	}

	if len(torrents) == 0 {
		xbmc.Notify("Quasar", "LOCALIZE[30205]", config.AddonIcon())
		return
	}

	AddToTorrentsMap(strconv.Itoa(episode.Id), torrents[0])

	rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[0].Magnet(),
	                                      "tmdb", strconv.Itoa(episode.Id),
	                                      "type", "episode",
	                                      "runtime", strconv.Itoa(runtime))
	ctx.Redirect(302, rUrl)
}
