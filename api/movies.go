package api

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/bittorrent"
	"github.com/scakemyer/quasar/providers"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/tmdb"
	"github.com/scakemyer/quasar/xbmc"
)

// Maps TMDB movie genre ids to slugs for images
var genreSlugs = map[int]string{
	28:    "action",
	10759: "action",
	12:    "adventure",
	16:    "animation",
	35:    "comedy",
	80:    "crime",
	99:    "documentary",
	18:    "drama",
	10761: "education",
	10751: "family",
	14:    "fantasy",
	10769: "foreign",
	36:    "history",
	27:    "horror",
	10762: "kids",
	10402: "music",
	9648:  "mystery",
	10763: "news",
	10764: "reality",
	10749: "romance",
	878:   "scifi",
	10765: "scifi",
	10766: "soap",
	10767: "talk",
	10770: "tv",
	53:    "thriller",
	10752: "war",
	10768: "war",
	37:    "western",
}

func MoviesIndex(ctx *gin.Context) {
	items := xbmc.ListItems{
		{Label: "LOCALIZE[30209]", Path: UrlForXBMC("/movies/search"), Thumbnail: config.AddonResource("img", "search.png")},
		{Label: "LOCALIZE[30056]", Path: UrlForXBMC("/movies/trakt/"), Thumbnail: config.AddonResource("img", "trakt.png")},
		{Label: "LOCALIZE[30210]", Path: UrlForXBMC("/movies/popular"), Thumbnail: config.AddonResource("img", "popular.png")},
		{Label: "LOCALIZE[30211]", Path: UrlForXBMC("/movies/top"), Thumbnail: config.AddonResource("img", "top_rated.png")},
		{Label: "LOCALIZE[30212]", Path: UrlForXBMC("/movies/mostvoted"), Thumbnail: config.AddonResource("img", "most_voted.png")},
		{Label: "LOCALIZE[30236]", Path: UrlForXBMC("/movies/recent"), Thumbnail: config.AddonResource("img", "clock.png")},
		{Label: "LOCALIZE[30213]", Path: UrlForXBMC("/movies/imdb250"), Thumbnail: config.AddonResource("img", "imdb.png")},
	}
	for _, genre := range tmdb.GetMovieGenres(config.Get().Language) {
		slug, _ := genreSlugs[genre.Id]
		items = append(items, &xbmc.ListItem{
			Label:     genre.Name,
			Path:      UrlForXBMC("/movies/popular/%s", strconv.Itoa(genre.Id)),
			Thumbnail: config.AddonResource("img", fmt.Sprintf("genre_%s.png", slug)),
			ContextMenu: [][]string{
				[]string{"LOCALIZE[30236]", fmt.Sprintf("Container.Update(%s)", UrlForXBMC("/movies/recent/%s", strconv.Itoa(genre.Id)))},
			},
		})
	}
	ctx.JSON(200, xbmc.NewView("", items))
}

func MoviesTrakt(ctx *gin.Context) {
	items := xbmc.ListItems{
		{Label: "LOCALIZE[30254]", Path: UrlForXBMC("/movies/trakt/watchlist"), Thumbnail: config.AddonResource("img", "trakt.png")},
		{Label: "LOCALIZE[30257]", Path: UrlForXBMC("/movies/trakt/collection"), Thumbnail: config.AddonResource("img", "trakt.png")},
		{Label: "LOCALIZE[30210]", Path: UrlForXBMC("/movies/trakt/popular"), Thumbnail: config.AddonResource("img", "popular.png")},
		{Label: "LOCALIZE[30246]", Path: UrlForXBMC("/movies/trakt/trending"), Thumbnail: config.AddonResource("img", "trending.png")},
		{Label: "LOCALIZE[30247]", Path: UrlForXBMC("/movies/trakt/played"), Thumbnail: config.AddonResource("img", "most_played.png")},
		{Label: "LOCALIZE[30248]", Path: UrlForXBMC("/movies/trakt/watched"), Thumbnail: config.AddonResource("img", "most_watched.png")},
		{Label: "LOCALIZE[30249]", Path: UrlForXBMC("/movies/trakt/collected"), Thumbnail: config.AddonResource("img", "most_collected.png")},
		{Label: "LOCALIZE[30250]", Path: UrlForXBMC("/movies/trakt/anticipated"), Thumbnail: config.AddonResource("img", "most_anticipated.png")},
		{Label: "LOCALIZE[30251]", Path: UrlForXBMC("/movies/trakt/boxoffice"), Thumbnail: config.AddonResource("img", "box_office.png")},
	}
	ctx.JSON(200, xbmc.NewView("", items))
}

func renderMovies(movies tmdb.Movies, ctx *gin.Context, page int, query string) {
	nextPage := 0
	if page >= 0 {
		nextPage = 1
	}
	items := make(xbmc.ListItems, 0, len(movies) + nextPage)
	for _, movie := range movies {
		if movie == nil {
			continue
		}
		item := movie.ToListItem()
		playUrl := UrlForXBMC("/movie/%d/play", movie.Id)
		movieLinksUrl := UrlForXBMC("/movie/%d/links", movie.Id)
		if config.Get().ChooseStreamAuto == true {
			item.Path = playUrl
		} else {
			item.Path = movieLinksUrl
		}

		libraryAction := []string{"LOCALIZE[30252]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/library/movie/add/%d", movie.Id))}
		if inJsonDb, err := InJsonDB(strconv.Itoa(movie.Id), LMovie); err == nil && inJsonDb == true {
			libraryAction = []string{"LOCALIZE[30253]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/library/movie/remove/%d", movie.Id))}
		}

		watchlistAction := []string{"LOCALIZE[30255]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/movie/%d/watchlist/add", movie.Id))}
		if InMoviesWatchlist(movie.Id) {
			watchlistAction = []string{"LOCALIZE[30256]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/movie/%d/watchlist/remove", movie.Id))}
		}

		collectionAction := []string{"LOCALIZE[30258]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/movie/%d/collection/add", movie.Id))}
		if InMoviesCollection(movie.Id) {
			collectionAction = []string{"LOCALIZE[30259]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/movie/%d/collection/remove", movie.Id))}
		}

		item.ContextMenu = [][]string{
			[]string{"LOCALIZE[30202]", fmt.Sprintf("XBMC.PlayMedia(%s)", movieLinksUrl)},
			[]string{"LOCALIZE[30023]", fmt.Sprintf("XBMC.PlayMedia(%s)", playUrl)},
			[]string{"LOCALIZE[30203]", "XBMC.Action(Info)"},
			libraryAction,
			watchlistAction,
			collectionAction,
			[]string{"LOCALIZE[30034]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/setviewmode/movies"))},
		}
		item.Info.Trailer = UrlForHTTP("/youtube/%s", item.Info.Trailer)
		item.IsPlayable = true
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
	ctx.JSON(200, xbmc.NewView("movies", items))
}

func PopularMovies(ctx *gin.Context) {
	genre := ctx.Params.ByName("genre")
	if genre == "0" {
		genre = ""
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	renderMovies(tmdb.PopularMovies(genre, config.Get().Language, page), ctx, page, "")
}

func RecentMovies(ctx *gin.Context) {
	genre := ctx.Params.ByName("genre")
	if genre == "0" {
		genre = ""
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	renderMovies(tmdb.RecentMovies(genre, config.Get().Language, page), ctx, page, "")
}

func TopRatedMovies(ctx *gin.Context) {
	genre := ctx.Params.ByName("genre")
	if genre == "0" {
		genre = ""
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	renderMovies(tmdb.TopRatedMovies(genre, config.Get().Language, page), ctx, page, "")
}

func IMDBTop250(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	renderMovies(tmdb.GetList("522effe419c2955e9922fcf3", config.Get().Language, page), ctx, page, "")
}

func MoviesMostVoted(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	renderMovies(tmdb.MostVotedMovies("", config.Get().Language, page), ctx, page, "")
}

func SearchMovies(ctx *gin.Context) {
	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		if len(searchHistory) > 0 && xbmc.DialogConfirm("Quasar", "LOCALIZE[30262]") {
			choice := xbmc.ListDialog("LOCALIZE[30261]", searchHistory...)
			query = searchHistory[choice]
		} else {
			query = xbmc.Keyboard("", "LOCALIZE[30206]")
			if query == "" {
				return
			}
			searchHistory = append(searchHistory, query)
		}
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	renderMovies(tmdb.SearchMovies(query, config.Get().Language, page), ctx, page, query)
}

func MovieGenres(ctx *gin.Context) {
	genres := tmdb.GetMovieGenres(config.Get().Language)
	items := make(xbmc.ListItems, 0, len(genres))
	for _, genre := range genres {
		items = append(items, &xbmc.ListItem{
			Label: genre.Name,
			Path:  UrlForXBMC("/movies/popular/%s", strconv.Itoa(genre.Id)),
		})
	}

	ctx.JSON(200, xbmc.NewView("", items))
}

func movieLinks(tmdbId string) []*bittorrent.Torrent {
	log.Println("Searching links for:", tmdbId)

	movie := tmdb.GetMovieById(tmdbId, config.Get().Language)

	log.Printf("Resolved %s to %s", tmdbId, movie.Title)

	if torrents := InTorrentsMap(tmdbId); len(torrents) > 0 {
		return torrents
	}

	searchers := providers.GetMovieSearchers()
	if len(searchers) == 0 {
		xbmc.Notify("Quasar", "LOCALIZE[30204]", config.AddonIcon())
	}

	return providers.SearchMovie(searchers, movie)
}

func MovieLinks(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("tmdbId")
	torrents := movieLinks(tmdbId)

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

	movie := tmdb.GetMovieById(tmdbId, config.Get().Language)
	runtime := 120
	if movie.Runtime > 0 {
		runtime = movie.Runtime
	}

	choice := xbmc.ListDialogLarge("LOCALIZE[30228]", movie.Title, choices...)
	if choice >= 0 {
		AddToTorrentsMap(tmdbId, torrents[choice])

		rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[choice].Magnet(),
		                                      "tmdb", tmdbId,
		                                      "type", "movie",
		                                      "runtime", strconv.Itoa(runtime))
		ctx.Redirect(302, rUrl)
	}
}

func MoviePlay(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("tmdbId")
	torrents := movieLinks(tmdbId)
	if len(torrents) == 0 {
		xbmc.Notify("Quasar", "LOCALIZE[30205]", config.AddonIcon())
		return
	}

	movie := tmdb.GetMovieById(tmdbId, "")
	runtime := 120
	if movie.Runtime > 0 {
		runtime = movie.Runtime
	}

	sort.Sort(sort.Reverse(providers.ByQuality(torrents)))

	AddToTorrentsMap(tmdbId, torrents[0])

	rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[0].Magnet(),
	                                      "tmdb", tmdbId,
	                                      "type", "movie",
	                                      "runtime", strconv.Itoa(runtime))
	ctx.Redirect(302, rUrl)
}
