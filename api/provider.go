package api

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/tmdb"
	"github.com/steeve/pulsar/tvdb"
)

type providerDebugResponse struct {
	Payload interface{} `json:"payload"`
	Results interface{} `json:"results"`
}

func ProviderGetMovie(ctx *gin.Context) {
	imdbId := ctx.Params.ByName("imdbId")
	provider := ctx.Params.ByName("provider")
	log.Println("Searching links for IMDB:", imdbId)
	movie := tmdb.GetMovieFromIMDB(imdbId, "en")
	log.Printf("Resolved %s to %s\n", imdbId, movie.Title)

	searcher := providers.NewAddonSearcher(provider)
	torrents := searcher.SearchMovieLinks(movie)
	if ctx.Request.URL.Query().Get("resolve") == "true" {
		for _, torrent := range torrents {
			torrent.Resolve()
		}
	}
	data, err := json.MarshalIndent(providerDebugResponse{
		Payload: searcher.GetMovieSearchObject(movie),
		Results: torrents,
	}, "", "    ")
	if err != nil {
		ctx.Error(err, nil)
	}
	ctx.Data(200, "application/json", data)
}

func ProviderGetEpisode(ctx *gin.Context) {
	provider := ctx.Params.ByName("provider")
	showId := ctx.Params.ByName("showId")
	seasonNumber, _ := strconv.Atoi(ctx.Params.ByName("season"))
	episodeNumber, _ := strconv.Atoi(ctx.Params.ByName("episode"))

	log.Println("Searching links for TVDB Id:", showId)

	show, err := tvdb.NewShowCached(showId, "en")
	if err != nil {
		ctx.Error(err, nil)
		return
	}
	episode := show.Seasons[seasonNumber].Episodes[episodeNumber-1]

	log.Printf("Resolved %s to %s\n", showId, show.SeriesName)

	searcher := providers.NewAddonSearcher(provider)
	torrents := searcher.SearchEpisodeLinks(show, episode)
	if ctx.Request.URL.Query().Get("resolve") == "true" {
		for _, torrent := range torrents {
			torrent.Resolve()
		}
	}
	data, err := json.MarshalIndent(providerDebugResponse{
		Payload: searcher.GetEpisodeSearchObject(show, episode),
		Results: torrents,
	}, "", "    ")
	if err != nil {
		ctx.Error(err, nil)
	}
	ctx.Data(200, "application/json", data)
}
