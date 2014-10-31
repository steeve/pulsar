package api

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/tmdb"
	"github.com/steeve/pulsar/xbmc"
)

func MoviesIndex(ctx *gin.Context) {
	items := xbmc.ListItems{
		{Label: "Search", Path: UrlForXBMC("/movies/search")},
		{Label: "Most Popular", Path: UrlForXBMC("/movies/popular")},
		{Label: "Top Rated", Path: UrlForXBMC("/movies/top")},
		{Label: "Most Voted", Path: UrlForXBMC("/movies/mostvoted")},
		{Label: "IMDB Top 250", Path: UrlForXBMC("/movies/imdb250")},
	}
	for _, genre := range tmdb.GetMovieGenres() {
		items = append(items, &xbmc.ListItem{
			Label: genre.Name,
			Path:  UrlForXBMC("/movies/popular/%s", strconv.Itoa(genre.Id)),
		})
	}
	ctx.JSON(200, xbmc.NewView("", items))
}

func renderMovies(movies tmdb.Movies, ctx *gin.Context) {
	items := make(xbmc.ListItems, 0, len(movies))
	for _, movie := range movies {
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
	renderMovies(tmdb.PopularMoviesComplete(genre), ctx)
}

func TopRatedMovies(ctx *gin.Context) {
	genre := ctx.Params.ByName("genre")
	if genre == "0" {
		genre = ""
	}
	renderMovies(tmdb.TopRatedMoviesComplete(genre), ctx)
}

func IMDBTop250(ctx *gin.Context) {
	renderMovies(tmdb.GetList("522effe419c2955e9922fcf3"), ctx)
}

func MoviesMostVoted(ctx *gin.Context) {
	renderMovies(tmdb.MostVotedMoviesComplete(""), ctx)
}

func SearchMovies(ctx *gin.Context) {
	query := ctx.Request.URL.Query().Get("q")
	if query == "" {
		query = xbmc.Keyboard("", "Search Movies")
	}
	renderMovies(tmdb.SearchMovies(query), ctx)
}

func MovieGenres(ctx *gin.Context) {
	genres := tmdb.GetMovieGenres()
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

	movie := tmdb.GetMovieFromIMDB(imdbId)

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
		xbmc.Notify("Pulsar", "No links were found.")
		return
	}
	sort.Sort(sort.Reverse(providers.ByQuality(torrents)))
	rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[0].Magnet())
	ctx.Redirect(302, rUrl)
}
