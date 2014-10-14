package api

import (
	"fmt"
	"net/url"
	"path"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steeve/pulsar/api/repository"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/cache"
	"github.com/steeve/pulsar/config"
	"github.com/steeve/pulsar/ga"
	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/util"
)

const (
	DefaultCacheTime    = 6 * time.Hour
	RepositoryCacheTime = 20 * time.Minute
	EpisodesCacheTime   = 15 * time.Minute
)

func Routes(btService *bittorrent.BTService) *gin.Engine {
	r := gin.Default()

	r.Use(ga.GATracker())

	store := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))

	r.GET("/", Index)
	r.GET("/search", Search)

	movies := r.Group("/movies")
	{
		movies.GET("/search", SearchMovies)
		movies.GET("/popular", cache.Cache(store, DefaultCacheTime), PopularMovies)
		movies.GET("/popular/:genre", cache.Cache(store, DefaultCacheTime), PopularMovies)
		movies.GET("/genres", MovieGenres)
	}
	movie := r.Group("/movie")
	{
		movie.GET("/:imdbId/links", MovieLinks)
		movie.GET("/:imdbId/play", MoviePlay)
	}

	shows := r.Group("/shows")
	{
		shows.GET("/search", SearchShows)
		shows.GET("/popular", cache.Cache(store, DefaultCacheTime), PopularShows)
	}
	show := r.Group("/show")
	{
		show.GET("/:showId/seasons", cache.Cache(store, DefaultCacheTime), ShowSeasons)
		show.GET("/:showId/season/:season/episodes", cache.Cache(store, EpisodesCacheTime), ShowEpisodes)
		show.GET("/:showId/season/:season/episode/:episode/links", ShowEpisodeLinks)
		show.GET("/:showId/season/:season/episode/:episode/play", ShowEpisodePlay)
	}

	repo := r.Group("/repository")
	{
		repo.GET("/:user/:repository/*filepath", repository.GetAddonFiles)
		repo.HEAD("/:user/:repository/*filepath", repository.GetAddonFiles)
	}

	r.GET("/play", Play(btService))
	r.POST("/callbacks/:cid", providers.CallbackHandler)

	return r
}

func UrlForHTTP(pattern string, args ...interface{}) string {
	u, _ := url.Parse(fmt.Sprintf(pattern, args...))
	return util.GetHTTPHost() + u.String()
}

func UrlForXBMC(pattern string, args ...interface{}) string {
	u, _ := url.Parse(fmt.Sprintf(pattern, args...))
	return "plugin://" + config.Get().Info.Id + u.String()
}

func UrlQuery(route string, query ...string) string {
	v := url.Values{}
	for i := 0; i < len(query); i += 2 {
		v.Add(query[i], query[i+1])
	}
	return route + "?" + v.Encode()
}
