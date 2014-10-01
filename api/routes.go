package api

import (
	"fmt"
	"net/url"
	"path"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/cache"
	"github.com/steeve/pulsar/config"
	"github.com/steeve/pulsar/ga"
	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/util"
)

const (
	DefaultCacheTime  = 6 * time.Hour
	EpisodesCacheTime = 15 * time.Minute
)

func Routes(btService *bittorrent.BTService) *gin.Engine {
	r := gin.Default()

	store := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))

	tracked := r.Group("/")
	tracked.Use(ga.GATracker())
	{
		tracked.GET("/", Index)
		tracked.GET("/search", Search)

		tracked.GET("/movies/search", SearchMovies)
		tracked.GET("/movies/popular", cache.Cache(store, DefaultCacheTime), PopularMovies)
		tracked.GET("/movies/popular/:genre", cache.Cache(store, DefaultCacheTime), PopularMovies)
		tracked.GET("/movies/genres", MovieGenres)
		tracked.GET("/movie/:imdbId/links", MovieLinks)
		tracked.GET("/movie/:imdbId/play", MoviePlay)

		tracked.GET("/shows/search", SearchShows)
		tracked.GET("/shows/popular", cache.Cache(store, DefaultCacheTime), PopularShows)
		tracked.GET("/show/:showId/seasons", cache.Cache(store, DefaultCacheTime), ShowSeasons)
		tracked.GET("/show/:showId/season/:season/episodes", cache.Cache(store, EpisodesCacheTime), ShowEpisodes)
		tracked.GET("/show/:showId/season/:season/episode/:episode/links", ShowEpisodeLinks)
		tracked.GET("/show/:showId/season/:season/episode/:episode/play", ShowEpisodePlay)
	}

	r.GET("/play", Play(btService))

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
