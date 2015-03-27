package api

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/config"
	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/tmdb"
	"github.com/steeve/pulsar/xbmc"
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
		{Label: "Search", Path: UrlForXBMC("/movies/search"), Thumbnail: AddonResource("img", "search.png")},
		{Label: "Most Popular", Path: UrlForXBMC("/movies/popular"), Thumbnail: AddonResource("img", "popular.png")},
		{Label: "Top Rated", Path: UrlForXBMC("/movies/top"), Thumbnail: AddonResource("img", "top_rated.png")},
		{Label: "Most Voted", Path: UrlForXBMC("/movies/mostvoted"), Thumbnail: AddonResource("img", "most_voted.png")},
		{Label: "IMDB Top 250", Path: UrlForXBMC("/movies/imdb250"), Thumbnail: AddonResource("img", "imdb.png")},
	}
	for _, genre := range tmdb.GetMovieGenres(config.Get().Language) {
		slug, _ := genreSlugs[genre.Id]
		items = append(items, &xbmc.ListItem{
			Label:     genre.Name,
			Path:      UrlForXBMC("/movies/popular/%s", strconv.Itoa(genre.Id)),
			Thumbnail: AddonResource("img", fmt.Sprintf("genre_%s.png", slug)),
		})
	}
	ctx.JSON(200, xbmc.NewView("", items))
}

func renderMovies(movies tmdb.Movies, ctx *gin.Context) {
	items := make(xbmc.ListItems, 0, len(movies))
	for _, movie := range movies {
		if movie == nil {
			continue
		}
		item := movie.ToListItem()
		item.Path = UrlForXBMC("/movie/%s/play", movie.IMDBId)
		item.IsPlayable = true
		item.ContextMenu = [][]string{
			[]string{"Choose stream...", fmt.Sprintf("XBMC.PlayMedia(%s)", UrlForXBMC("/movie/%s/links", movie.IMDBId))},
		}
		items = append(items, item)
	}

	ctx.JSON(200, xbmc.NewView("movies", items))
}

func PopularMovies(ctx *gin.Context) {
	genre := ctx.Params.ByName("genre")
	if genre == "0" {
		genre = ""
	}
	renderMovies(tmdb.PopularMoviesComplete(genre, config.Get().Language), ctx)
}

func TopRatedMovies(ctx *gin.Context) {
	genre := ctx.Params.ByName("genre")
	if genre == "0" {
		genre = ""
	}
	renderMovies(tmdb.TopRatedMoviesComplete(genre, config.Get().Language), ctx)
}

func IMDBTop250(ctx *gin.Context) {
	renderMovies(tmdb.GetList("522effe419c2955e9922fcf3", config.Get().Language), ctx)
}

func MoviesMostVoted(ctx *gin.Context) {
	renderMovies(tmdb.MostVotedMoviesComplete("", config.Get().Language), ctx)
}

func SearchMovies(ctx *gin.Context) {
	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		query = xbmc.Keyboard("", "Search Movies")
	}
	renderMovies(tmdb.SearchMovies(query, config.Get().Language), ctx)
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
		xbmc.Notify("Pulsar", "Unable to find any providers")
	}

	return providers.SearchMovie(searchers, movie)
}

func MovieLinks(ctx *gin.Context) {
	torrents := movieLinks(ctx.Params.ByName("imdbId"))

	if len(torrents) == 0 {
		xbmc.Notify("Pulsar", "No links were found")
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

	choice := xbmc.ListDialog("Choose stream", choices...)
	if choice >= 0 {
		rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[choice].Magnet())
		ctx.Redirect(302, rUrl)
	}
}

func MoviePlay(ctx *gin.Context) {
	torrents := movieLinks(ctx.Params.ByName("imdbId"))
	if len(torrents) == 0 {
		xbmc.Notify("Pulsar", "No links were found")
		return
	}
	sort.Sort(sort.Reverse(providers.ByQuality(torrents)))
	rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[0].Magnet())
	ctx.Redirect(302, rUrl)
}
