package api

import (
	"fmt"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/ga"
	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/util"
)

const (
	PREFIX = "plugin://plugin.video.pulsar"
)

func Routes(btService *bittorrent.BTService) *gin.Engine {
	r := gin.Default()

	r.Use(ga.GATracker())

	r.GET("/", Index)
	r.GET("/search", Search)

	r.GET("/movies/search", SearchMovies)
	r.GET("/movies/popular", PopularMovies)
	r.GET("/movies/popular/:genre", PopularMovies)
	r.GET("/movies/genres", MovieGenres)
	r.GET("/movie/:imdbId/links", MovieLinks)
	r.GET("/movie/:imdbId/play", MoviePlay)

	r.GET("/shows/search", SearchShows)
	r.GET("/shows/popular", PopularShows)
	r.GET("/show/:showId/seasons", ShowSeasons)
	r.GET("/show/:showId/season/:season/episodes", ShowEpisodes)
	r.GET("/show/:showId/season/:season/episode/:episode/links", ShowEpisodeLinks)
	r.GET("/show/:showId/season/:season/episode/:episode/play", ShowEpisodePlay)

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
	return PREFIX + u.String()
}

func UrlQuery(route string, query ...string) string {
	v := url.Values{}
	for i := 0; i < len(query); i += 2 {
		v.Add(query[i], query[i+1])
	}
	return route + "?" + v.Encode()
}

// func UrlFor(name string, args ...string) string {
// 	url, err := routes.Get(name).URLPath(args...)
// 	if err != nil {
// 		fmt.Println(err)
// 		return ""
// 	}
// 	return url.String()
// }

// func UrlForHTTP(name string, args ...string) string {
// 	return util.GetHTTPHost() + UrlFor(name, args...)
// }

// func UrlForXBMC(name string, args ...string) string {
// 	return PREFIX + UrlFor(name, args...)
// }
