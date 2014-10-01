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

	searchers := make([]providers.MovieSearcher, 0)
	for _, addon := range xbmc.GetAddons("xbmc.python.script").Addons {
		if strings.HasPrefix(addon.ID, "script.pulsar.") {
			searchers = append(searchers, providers.NewAddonSearcher(addon.ID))
		}
	}

	return providers.SearchMovie(searchers, movie)
}

func MovieLinks(ctx *gin.Context) {
	torrents := movieLinks(ctx.Params.ByName("imdbId"))
	choices := make([]string, 0, len(torrents))
	for _, torrent := range torrents {
		info := make([]string, 0, 4)
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
		rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[choice].URI)
		ctx.Writer.Header().Set("Location", rUrl)
		ctx.Abort(302)
	}
}

func MoviePlay(ctx *gin.Context) {
	torrents := movieLinks(ctx.Params.ByName("imdbId"))
	sort.Sort(sort.Reverse(providers.ByQuality(torrents)))
	rUrl := UrlQuery(UrlForXBMC("/play"), "uri", torrents[0].URI)
	ctx.Writer.Header().Set("Location", rUrl)
	ctx.Abort(302)
}
