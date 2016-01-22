package api

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/pulsar/bittorrent"
	"github.com/scakemyer/pulsar/config"
	"github.com/scakemyer/pulsar/providers"
	"github.com/scakemyer/pulsar/tmdb"
	"github.com/scakemyer/pulsar/xbmc"
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
		{Label: "LOCALIZE[30210]", Path: UrlForXBMC("/movies/popular"), Thumbnail: config.AddonResource("img", "popular.png")},
		{Label: "LOCALIZE[30211]", Path: UrlForXBMC("/movies/top"), Thumbnail: config.AddonResource("img", "top_rated.png")},
		{Label: "LOCALIZE[30212]", Path: UrlForXBMC("/movies/mostvoted"), Thumbnail: config.AddonResource("img", "most_voted.png")},
		{Label: "LOCALIZE[30213]", Path: UrlForXBMC("/movies/imdb250"), Thumbnail: config.AddonResource("img", "imdb.png")},
	}
	for _, genre := range tmdb.GetMovieGenres(config.Get().Language) {
		slug, _ := genreSlugs[genre.Id]
		items = append(items, &xbmc.ListItem{
			Label:     genre.Name,
			Path:      UrlForXBMC("/movies/popular/%s", strconv.Itoa(genre.Id)),
			Thumbnail: config.AddonResource("img", fmt.Sprintf("genre_%s.png", slug)),
		})
	}
	ctx.JSON(200, xbmc.NewView("", items))
}

func renderMovies(movies tmdb.Movies, ctx *gin.Context, page int) {
	paging := 0
	if page >= 0 {
		paging = 1
	}
	items := make(xbmc.ListItems, 0, len(movies) + paging)
	for _, movie := range movies {
		if movie == nil {
			continue
		}
		item := movie.ToListItem()
		item.Path = UrlForXBMC("/movie/%s/play", movie.IMDBId)
		item.Info.Trailer = UrlForHTTP("/youtube/%s", item.Info.Trailer)
		item.IsPlayable = true
		item.ContextMenu = [][]string{
			[]string{"LOCALIZE[30202]", fmt.Sprintf("XBMC.PlayMedia(%s)", UrlForXBMC("/movie/%s/links", movie.IMDBId))},
			[]string{"LOCALIZE[30203]", "XBMC.Action(Info)"},
		}
		items = append(items, item)
	}
	if page >= 0 {
		path := ctx.Request.URL.Path 
		nextpage := &xbmc.ListItem{Label: "LOCALIZE[30218]", Path: UrlForXBMC(fmt.Sprintf("%s?page=%d", path, page + 1)), Thumbnail: config.AddonResource("img", "nextpage.png")}
		items = append(items, nextpage)
	}
	ctx.JSON(200, xbmc.NewView("movies", items))
}

func PopularMovies(ctx *gin.Context) {
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
	renderMovies(tmdb.PopularMoviesComplete(genre, config.Get().Language, page), ctx, page)
}

func TopRatedMovies(ctx *gin.Context) {
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
	renderMovies(tmdb.TopRatedMoviesComplete(genre, config.Get().Language, page), ctx, page)
}

func IMDBTop250(ctx *gin.Context) {
	renderMovies(tmdb.GetList("522effe419c2955e9922fcf3", config.Get().Language), ctx, -1)
}

func MoviesMostVoted(ctx *gin.Context) {
	page := -1
	if config.Get().EnablePaging == true {
		currentpage, err := strconv.Atoi(ctx.DefaultQuery("page", "0"))
    	if err == nil {
			page = currentpage
		}
	}
	renderMovies(tmdb.MostVotedMoviesComplete("", config.Get().Language, page), ctx, page)
}

func SearchMovies(ctx *gin.Context) {
	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		query = xbmc.Keyboard("", "LOCALIZE[30206]")
		if query == "" {
			return
		}
	}
	renderMovies(tmdb.SearchMovies(query, config.Get().Language), ctx, -1)
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

func movieLinks(imdbId string) []*bittorrent.Torrent {
	log.Println("Searching links for IMDB:", imdbId)

	movie := tmdb.GetMovieFromIMDB(imdbId, config.Get().Language)

	log.Printf("Resolved %s to %s\n", imdbId, movie.Title)

	searchers := providers.GetMovieSearchers()
	if len(searchers) == 0 {
		xbmc.Notify("Pulsar", "LOCALIZE[30204]", config.AddonIcon())
	}

	return providers.SearchMovie(searchers, movie)
}

func MovieLinks(ctx *gin.Context) {
	torrents := movieLinks(ctx.Params.ByName("imdbId"))

	if len(torrents) == 0 {
		xbmc.Notify("Pulsar", "LOCALIZE[30205]", config.AddonIcon())
		return
	}

	choices := make([]string, 0, len(torrents))
	for _, torrent := range torrents {
		info := make([]string, 0)
		if torrent.RipType > 0 {
			info = append(info, bittorrent.Rips[torrent.RipType])
		}
		if torrent.Resolution > 0 {
			info = append(info, bittorrent.Resolutions[torrent.Resolution])
		}
		if torrent.VideoCodec > 0 {
			info = append(info, bittorrent.Codecs[torrent.VideoCodec])
		}
		if torrent.AudioCodec > 0 {
			info = append(info, bittorrent.Codecs[torrent.AudioCodec])
		}

		label := fmt.Sprintf("S:%d P:%d - %s - %s",
			torrent.Seeds,
			torrent.Peers,
			strings.Join(info, " "),
			torrent.Name,
		)
		choices = append(choices, label)
	}

	choice := xbmc.ListDialog("LOCALIZE[30202]", choices...)
	if choice >= 0 {
		rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[choice].Magnet())
		ctx.Redirect(302, rUrl)
	}
}

func MoviePlay(ctx *gin.Context) {
	torrents := movieLinks(ctx.Params.ByName("imdbId"))
	if len(torrents) == 0 {
		xbmc.Notify("Pulsar", "LOCALIZE[30205]", config.AddonIcon())
		return
	}
	sort.Sort(sort.Reverse(providers.ByQuality(torrents)))
	rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[0].Magnet())
	ctx.Redirect(302, rUrl)
}
