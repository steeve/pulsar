package api

import (
	"fmt"
	"net/url"
	"path"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/api/repository"
	"github.com/scakemyer/quasar/bittorrent"
	"github.com/scakemyer/quasar/cache"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/providers"
	"github.com/scakemyer/quasar/util"
)

const (
	DefaultCacheTime    = 6 * time.Hour
	RepositoryCacheTime = 20 * time.Minute
	EpisodesCacheTime   = 15 * time.Minute
	IndexCacheTime      = 15 * 24 * time.Hour // 15 days caching for index
)

func Routes(btService *bittorrent.BTService) *gin.Engine {
	r := gin.Default()

	gin.SetMode(gin.ReleaseMode)

	//r.Use(ga.GATracker())

	store := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))

	r.GET("/", Index)
	r.GET("/search", Search)
	r.GET("/pasted", PasteURL)

	movies := r.Group("/movies")
	{
		movies.GET("/", cache.Cache(store, IndexCacheTime), MoviesIndex)
		movies.GET("/search", SearchMovies)
		movies.GET("/popular", cache.Cache(store, DefaultCacheTime), PopularMovies)
		movies.GET("/popular/:genre", cache.Cache(store, DefaultCacheTime), PopularMovies)
		movies.GET("/top", cache.Cache(store, DefaultCacheTime), TopRatedMovies)
		movies.GET("/imdb250", cache.Cache(store, DefaultCacheTime), IMDBTop250)
		movies.GET("/mostvoted", cache.Cache(store, DefaultCacheTime), MoviesMostVoted)
		movies.GET("/genres", cache.Cache(store, IndexCacheTime), MovieGenres)
	}
	movie := r.Group("/movie")
	{
		movie.GET("/:imdbId/links", MovieLinks)
		movie.GET("/:imdbId/play", MoviePlay)
	}

	shows := r.Group("/shows")
	{
		shows.GET("/", cache.Cache(store, IndexCacheTime), TVIndex)
		shows.GET("/search", SearchShows)
		shows.GET("/popular", cache.Cache(store, DefaultCacheTime), PopularShows)
		shows.GET("/popular/:genre", cache.Cache(store, DefaultCacheTime), PopularShows)
		shows.GET("/top", cache.Cache(store, DefaultCacheTime), TopRatedShows)
		shows.GET("/mostvoted", cache.Cache(store, DefaultCacheTime), TVMostVoted)
		shows.GET("/genres", cache.Cache(store, IndexCacheTime), TVGenres)
	}
	show := r.Group("/show")
	{
		show.GET("/:showId/seasons", cache.Cache(store, DefaultCacheTime), ShowSeasons)
		show.GET("/:showId/season/:season/episodes", cache.Cache(store, EpisodesCacheTime), ShowEpisodes)
		show.GET("/:showId/season/:season/episode/:episode/links", ShowEpisodeLinks)
		show.GET("/:showId/season/:season/episode/:episode/play", ShowEpisodePlay)
	}

	provider := r.Group("/provider")
	{
		provider.GET("/:provider/movie/:imdbId", ProviderGetMovie)
		provider.GET("/:provider/show/:showId/season/:season/episode/:episode", ProviderGetEpisode)
	}

	repo := r.Group("/repository")
	{
		repo.GET("/:user/:repository/*filepath", repository.GetAddonFiles)
		repo.HEAD("/:user/:repository/*filepath", repository.GetAddonFiles)
	}

	r.GET("/youtube/:id", PlayYoutubeVideo)

	r.GET("/subtitles", SubtitlesIndex)
	r.GET("/subtitle/:id", SubtitleGet)

	r.GET("/play", Play(btService))
	r.POST("/callbacks/:cid", providers.CallbackHandler)

	cmd := r.Group("/cmd")
	{
		cmd.GET("/clear_cache", ClearCache)
	}

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
