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

	store := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))

	r.GET("/", Index)
	r.GET("/search", Search)
	r.GET("/pasted", PasteURL)

	torrents := r.Group("/torrents")
	{
		torrents.GET("/", ListTorrents(btService))
		torrents.GET("/pause", PauseSession(btService))
		torrents.GET("/resume", ResumeSession(btService))
		torrents.GET("/pause/:torrentId", PauseTorrent(btService))
		torrents.GET("/resume/:torrentId", ResumeTorrent(btService))
		torrents.GET("/delete/:torrentId", RemoveTorrent(btService))
	}

	movies := r.Group("/movies")
	{
		movies.GET("/", cache.Cache(store, IndexCacheTime), MoviesIndex)
		movies.GET("/search", SearchMovies)
		movies.GET("/popular", cache.Cache(store, DefaultCacheTime), PopularMovies)
		movies.GET("/popular/:genre", cache.Cache(store, DefaultCacheTime), PopularMovies)
		movies.GET("/recent", cache.Cache(store, DefaultCacheTime), RecentMovies)
		movies.GET("/recent/:genre", cache.Cache(store, DefaultCacheTime), RecentMovies)
		movies.GET("/top", cache.Cache(store, DefaultCacheTime), TopRatedMovies)
		movies.GET("/imdb250", cache.Cache(store, DefaultCacheTime), IMDBTop250)
		movies.GET("/mostvoted", cache.Cache(store, DefaultCacheTime), MoviesMostVoted)
		movies.GET("/genres", cache.Cache(store, IndexCacheTime), MovieGenres)

		trakt := movies.Group("/trakt")
		{
			trakt.GET("/", cache.Cache(store, IndexCacheTime), MoviesTrakt)
			trakt.GET("/watchlist", WatchlistMovies)
			trakt.GET("/collection", CollectionMovies)
			trakt.GET("/popular", cache.Cache(store, DefaultCacheTime), TraktPopularMovies)
			trakt.GET("/trending", cache.Cache(store, DefaultCacheTime), TraktTrendingMovies)
			trakt.GET("/played", cache.Cache(store, DefaultCacheTime), TraktMostPlayedMovies)
			trakt.GET("/watched", cache.Cache(store, DefaultCacheTime), TraktMostWatchedMovies)
			trakt.GET("/collected", cache.Cache(store, DefaultCacheTime), TraktMostCollectedMovies)
			trakt.GET("/anticipated", cache.Cache(store, DefaultCacheTime), TraktMostAnticipatedMovies)
			trakt.GET("/boxoffice", cache.Cache(store, DefaultCacheTime), TraktBoxOffice)
		}
	}
	movie := r.Group("/movie")
	{
		movie.GET("/:tmdbId/links", MovieLinks)
		movie.GET("/:tmdbId/play", MoviePlay)
		movie.GET("/:tmdbId/watchlist/add", AddMovieToWatchlist)
		movie.GET("/:tmdbId/watchlist/remove", RemoveMovieFromWatchlist)
		movie.GET("/:tmdbId/collection/add", AddMovieToCollection)
		movie.GET("/:tmdbId/collection/remove", RemoveMovieFromCollection)
	}

	shows := r.Group("/shows")
	{
		shows.GET("/", cache.Cache(store, IndexCacheTime), TVIndex)
		shows.GET("/search", SearchShows)
		shows.GET("/popular", cache.Cache(store, DefaultCacheTime), PopularShows)
		shows.GET("/popular/:genre", cache.Cache(store, DefaultCacheTime), PopularShows)
		shows.GET("/recent/shows", cache.Cache(store, DefaultCacheTime), RecentShows)
		shows.GET("/recent/shows/:genre", cache.Cache(store, DefaultCacheTime), RecentShows)
		shows.GET("/recent/episodes", cache.Cache(store, DefaultCacheTime), RecentEpisodes)
		shows.GET("/recent/episodes/:genre", cache.Cache(store, DefaultCacheTime), RecentEpisodes)
		shows.GET("/top", cache.Cache(store, DefaultCacheTime), TopRatedShows)
		shows.GET("/mostvoted", cache.Cache(store, DefaultCacheTime), TVMostVoted)
		shows.GET("/genres", cache.Cache(store, IndexCacheTime), TVGenres)

		trakt := shows.Group("/trakt")
		{
			trakt.GET("/", cache.Cache(store, IndexCacheTime), TVTrakt)
			trakt.GET("/watchlist", WatchlistShows)
			trakt.GET("/collection", CollectionShows)
			trakt.GET("/popular", cache.Cache(store, DefaultCacheTime), TraktPopularShows)
			trakt.GET("/trending", cache.Cache(store, DefaultCacheTime), TraktTrendingShows)
			trakt.GET("/played", cache.Cache(store, DefaultCacheTime), TraktMostPlayedShows)
			trakt.GET("/watched", cache.Cache(store, DefaultCacheTime), TraktMostWatchedShows)
			trakt.GET("/collected", cache.Cache(store, DefaultCacheTime), TraktMostCollectedShows)
			trakt.GET("/anticipated", cache.Cache(store, DefaultCacheTime), TraktMostAnticipatedShows)
		}
	}
	show := r.Group("/show")
	{
		show.GET("/:showId/seasons", cache.Cache(store, DefaultCacheTime), ShowSeasons)
		show.GET("/:showId/season/:season/links", ShowSeasonLinks)
		show.GET("/:showId/season/:season/episodes", cache.Cache(store, EpisodesCacheTime), ShowEpisodes)
		show.GET("/:showId/season/:season/episode/:episode/play", ShowEpisodePlay)
		show.GET("/:showId/season/:season/episode/:episode/links", ShowEpisodeLinks)
		show.GET("/:showId/watchlist/add", AddShowToWatchlist)
		show.GET("/:showId/watchlist/remove", RemoveShowFromWatchlist)
		show.GET("/:showId/collection/add", AddShowToCollection)
		show.GET("/:showId/collection/remove", RemoveShowFromCollection)
	}
	// TODO
	// episode := r.Group("/episode")
	// {
	// 	episode.GET("/:episodeId/watchlist/add", AddEpisodeToWatchlist)
	// }

	library := r.Group("/library")
	{
		library.GET("/movie/add/:tmdbId", AddMovie)
		library.GET("/movie/remove/:tmdbId", RemoveMovie)
		library.GET("/show/add/:showId", AddShow)
		library.GET("/show/remove/:showId", RemoveShow)
		library.GET("/update", UpdateLibrary)
		library.GET("/getpath", GetLibraryPath)
		library.GET("/getcount", GetCount)
		library.GET("/lookup", Lookup)
		library.GET("/play/movie/:tmdbId", PlayMovie)
		library.GET("/play/show/:showId/season/:season/episode/:episode", PlayShow)
	}

	provider := r.Group("/provider")
	{
		provider.GET("/", ProviderList)
		provider.GET("/:provider/check", ProviderCheck)
		provider.GET("/:provider/enable", ProviderEnable)
		provider.GET("/:provider/disable", ProviderDisable)
		provider.GET("/:provider/failure", ProviderFailure)
		provider.GET("/:provider/settings", ProviderSettings)

		provider.GET("/:provider/movie/:tmdbId", ProviderGetMovie)
		provider.GET("/:provider/show/:showId/season/:season/episode/:episode", ProviderGetEpisode)
	}

	repo := r.Group("/repository")
	{
		repo.GET("/:user/:repository/*filepath", repository.GetAddonFiles)
		repo.HEAD("/:user/:repository/*filepath", repository.GetAddonFilesHead)
	}

	trakt := r.Group("/trakt")
	{
		trakt.GET("/authorize", AuthorizeTrakt)
	}

	r.GET("/setviewmode/:content_type", SetViewMode)

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
