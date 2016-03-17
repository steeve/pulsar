package api

import (
	"os"
	"fmt"
	"path"
	"time"
	"strconv"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/cache"
	"github.com/scakemyer/quasar/trakt"
	"github.com/scakemyer/quasar/xbmc"
)

func AuthorizeTrakt(ctx *gin.Context) {
	err := trakt.Authorize(true)
	if err == nil {
		ctx.String(200, "")
	} else {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
		ctx.String(200, "")
	}
}

func WatchlistMovies(ctx *gin.Context) {
	movies, err := trakt.WatchlistMovies()
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktMovies(movies, ctx, 0)
}

func WatchlistShows(ctx *gin.Context) {
	shows, err := trakt.WatchlistShows()
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktShows(shows, ctx, 0)
}

func CollectionMovies(ctx *gin.Context) {
	movies, err := trakt.CollectionMovies()
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktMovies(movies, ctx, 0)
}

func CollectionShows(ctx *gin.Context) {
	shows, err := trakt.CollectionShows()
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktShows(shows, ctx, 0)
}

// func WatchlistSeasons(ctx *gin.Context) {
// 	renderTraktSeasons(trakt.Watchlist("seasons", pageParam), ctx, page)
// }

// func WatchlistEpisodes(ctx *gin.Context) {
// 	renderTraktEpisodes(trakt.Watchlist("episodes", pageParam), ctx, page)
// }

func AddMovieToWatchlist(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("tmdbId")
	resp, err := trakt.AddToWatchlist("movies", tmdbId)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	} else if resp.Status() != 201 {
		xbmc.Notify("Quasar", fmt.Sprintf("Failed with %d status code", resp.Status()), config.AddonIcon())
	} else {
		xbmc.Notify("Quasar", "Movie added to watchlist", config.AddonIcon())
		os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
		xbmc.Refresh()
	}
}

func RemoveMovieFromWatchlist(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("tmdbId")
	resp, err := trakt.RemoveFromWatchlist("movies", tmdbId)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	} else if resp.Status() != 200 {
		xbmc.Notify("Quasar", fmt.Sprintf("Failed with %d status code", resp.Status()), config.AddonIcon())
	} else {
		xbmc.Notify("Quasar", "Movie removed from watchlist", config.AddonIcon())
		os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
		xbmc.Refresh()
	}
}

func AddShowToWatchlist(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("showId")
	resp, err := trakt.AddToWatchlist("shows", tmdbId)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	} else if resp.Status() != 201 {
		xbmc.Notify("Quasar", fmt.Sprintf("Failed %d", resp.Status()), config.AddonIcon())
	} else {
		xbmc.Notify("Quasar", "Show added to watchlist", config.AddonIcon())
		os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
		xbmc.Refresh()
	}
}

func RemoveShowFromWatchlist(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("showId")
	resp, err := trakt.RemoveFromWatchlist("shows", tmdbId)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	} else if resp.Status() != 200 {
		xbmc.Notify("Quasar", fmt.Sprintf("Failed with %d status code", resp.Status()), config.AddonIcon())
	} else {
		xbmc.Notify("Quasar", "Show removed from watchlist", config.AddonIcon())
		os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
		xbmc.Refresh()
	}
}

func AddMovieToCollection(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("tmdbId")
	resp, err := trakt.AddToCollection("movies", tmdbId)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	} else if resp.Status() != 201 {
		xbmc.Notify("Quasar", fmt.Sprintf("Failed with %d status code", resp.Status()), config.AddonIcon())
	} else {
		xbmc.Notify("Quasar", "Movie added to collection", config.AddonIcon())
		os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
		xbmc.Refresh()
	}
}

func RemoveMovieFromCollection(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("tmdbId")
	resp, err := trakt.RemoveFromCollection("movies", tmdbId)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	} else if resp.Status() != 200 {
		xbmc.Notify("Quasar", fmt.Sprintf("Failed with %d status code", resp.Status()), config.AddonIcon())
	} else {
		xbmc.Notify("Quasar", "Movie removed from collection", config.AddonIcon())
		os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
		xbmc.Refresh()
	}
}

func AddShowToCollection(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("showId")
	resp, err := trakt.AddToCollection("shows", tmdbId)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	} else if resp.Status() != 201 {
		xbmc.Notify("Quasar", fmt.Sprintf("Failed with %d status code", resp.Status()), config.AddonIcon())
	} else {
		xbmc.Notify("Quasar", "Show added to collection", config.AddonIcon())
		os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
		xbmc.Refresh()
	}
}

func RemoveShowFromCollection(ctx *gin.Context) {
	tmdbId := ctx.Params.ByName("showId")
	resp, err := trakt.RemoveFromCollection("shows", tmdbId)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	} else if resp.Status() != 200 {
		xbmc.Notify("Quasar", fmt.Sprintf("Failed with %d status code", resp.Status()), config.AddonIcon())
	} else {
		xbmc.Notify("Quasar", "Show removed from collection", config.AddonIcon())
		os.RemoveAll(filepath.Join(config.Get().Info.Profile, "cache"))
		xbmc.Refresh()
	}
}

// func AddEpisodeToWatchlist(ctx *gin.Context) {
// 	tmdbId := ctx.Params.ByName("episodeId")
// 	resp, err := trakt.AddToWatchlist("episodes", tmdbId)
// 	if err != nil {
// 		xbmc.Notify("Quasar", fmt.Sprintf("Failed: %s", err), config.AddonIcon())
// 	} else if resp.Status() != 201 {
// 		xbmc.Notify("Quasar", fmt.Sprintf("Failed: %d", resp.Status()), config.AddonIcon())
// 	} else {
// 		xbmc.Notify("Quasar", "Episode added to watchlist", config.AddonIcon())
// 	}
// }

func InMoviesWatchlist(tmdbId int) bool {
	if config.Get().TraktToken == "" {
		return false
	}

	var movies []*trakt.Movies

	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := fmt.Sprintf("com.trakt.watchlist.movies")
	if err := cacheStore.Get(key, &movies); err != nil {
		movies, _ := trakt.WatchlistMovies()
		cacheStore.Set(key, movies, 30 * time.Second)
	}

	for _, movie := range movies {
		if tmdbId == movie.Movie.IDs.TMDB {
			return true
		}
	}
	return false
}

func InShowsWatchlist(tmdbId int) bool {
	if config.Get().TraktToken == "" {
		return false
	}

	var shows []*trakt.Shows

	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := fmt.Sprintf("com.trakt.watchlist.shows")
	if err := cacheStore.Get(key, &shows); err != nil {
		shows, _ := trakt.WatchlistShows()
		cacheStore.Set(key, shows, 30 * time.Second)
	}

	for _, show := range shows {
		if tmdbId == show.Show.IDs.TMDB {
			return true
		}
	}
	return false
}

func InMoviesCollection(tmdbId int) bool {
	if config.Get().TraktToken == "" {
		return false
	}

	var movies []*trakt.Movies

	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := fmt.Sprintf("com.trakt.collection.movies")
	if err := cacheStore.Get(key, &movies); err != nil {
		movies, _ := trakt.CollectionMovies()
		cacheStore.Set(key, movies, 30 * time.Second)
	}

	for _, movie := range movies {
		if tmdbId == movie.Movie.IDs.TMDB {
			return true
		}
	}
	return false
}

func InShowsCollection(tmdbId int) bool {
	if config.Get().TraktToken == "" {
		return false
	}

	var shows []*trakt.Shows

	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := fmt.Sprintf("com.trakt.collection.shows")
	if err := cacheStore.Get(key, &shows); err != nil {
		shows, _ := trakt.CollectionShows()
		cacheStore.Set(key, shows, 30 * time.Second)
	}

	for _, show := range shows {
		if tmdbId == show.Show.IDs.TMDB {
			return true
		}
	}
	return false
}

func renderTraktMovies(movies []*trakt.Movies, ctx *gin.Context, page int) {
	hasNextPage := 0
	if page > 0 {
		hasNextPage = 1
	}

	items := make(xbmc.ListItems, 0, len(movies) + hasNextPage)

	for _, movieListing := range movies {
    movie := movieListing.Movie
		if movie == nil {
			continue
		}
		item := movie.ToListItem()
		playUrl := UrlForXBMC("/movie/%d/play", movie.IDs.TMDB)
		movieLinksUrl := UrlForXBMC("/movie/%d/links", movie.IDs.TMDB)
		if config.Get().ChooseStreamAuto == true {
			item.Path = playUrl
		} else {
			item.Path = movieLinksUrl
		}

		libraryAction := []string{"LOCALIZE[30252]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/library/movie/add/%d", movie.IDs.TMDB))}
		if inJsonDb, err := InJsonDB(strconv.Itoa(movie.IDs.TMDB), LMovie); err == nil && inJsonDb == true {
			libraryAction = []string{"LOCALIZE[30253]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/library/movie/remove/%d", movie.IDs.TMDB))}
		}

		watchlistAction := []string{"LOCALIZE[30255]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/movie/%d/watchlist/add", movie.IDs.TMDB))}
		if InMoviesWatchlist(movie.IDs.TMDB) {
			watchlistAction = []string{"LOCALIZE[30256]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/movie/%d/watchlist/remove", movie.IDs.TMDB))}
		}

		collectionAction := []string{"LOCALIZE[30258]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/movie/%d/collection/add", movie.IDs.TMDB))}
		if InMoviesCollection(movie.IDs.TMDB) {
			collectionAction = []string{"LOCALIZE[30259]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/movie/%d/collection/remove", movie.IDs.TMDB))}
		}

		item.ContextMenu = [][]string{
			[]string{"LOCALIZE[30202]", fmt.Sprintf("XBMC.PlayMedia(%s)", movieLinksUrl)},
			[]string{"LOCALIZE[30023]", fmt.Sprintf("XBMC.PlayMedia(%s)", playUrl)},
			[]string{"LOCALIZE[30203]", "XBMC.Action(Info)"},
			libraryAction,
			watchlistAction,
			collectionAction,
			[]string{"LOCALIZE[30034]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/setviewmode/movies"))},
		}
		// item.Info.Trailer = UrlForHTTP("/youtube/%s", movie.Trailer)
		item.IsPlayable = true
		items = append(items, item)
	}
	if page >= 0 && hasNextPage > 0 {
		path := ctx.Request.URL.Path
		nextpage := &xbmc.ListItem{
			Label: "LOCALIZE[30218]",
			Path: UrlForXBMC(fmt.Sprintf("%s?page=%d", path, page + 1)),
			Thumbnail: config.AddonResource("img", "nextpage.png"),
		}
		items = append(items, nextpage)
	}
	ctx.JSON(200, xbmc.NewView("movies", items))
}

func TraktPopularMovies(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	movies, err := trakt.TopMovies("popular", pageParam)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktMovies(movies, ctx, page)
}

func TraktTrendingMovies(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	movies, err := trakt.TopMovies("trending", pageParam)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktMovies(movies, ctx, page)
}

func TraktMostPlayedMovies(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	movies, err := trakt.TopMovies("played", pageParam)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktMovies(movies, ctx, page)
}

func TraktMostWatchedMovies(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	movies, err := trakt.TopMovies("watched", pageParam)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktMovies(movies, ctx, page)
}

func TraktMostCollectedMovies(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	movies, err := trakt.TopMovies("collected", pageParam)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktMovies(movies, ctx, page)
}

func TraktMostAnticipatedMovies(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	movies, err := trakt.TopMovies("anticipated", pageParam)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktMovies(movies, ctx, page)
}

func TraktBoxOffice(ctx *gin.Context) {
	movies, err := trakt.TopMovies("boxoffice", "1")
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktMovies(movies, ctx, 0)
}


func renderTraktShows(shows []*trakt.Shows, ctx *gin.Context, page int) {
	hasNextPage := 0
	if page > 0 {
		hasNextPage = 1
	}

	items := make(xbmc.ListItems, 0, len(shows) + hasNextPage)

	for _, showListing := range shows {
    show := showListing.Show
		if show == nil {
			continue
		}
		item := show.ToListItem()
		item.Path = UrlForXBMC("/show/%d/seasons", show.IDs.TMDB)

		libraryAction := []string{"LOCALIZE[30252]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/library/show/add/%d", show.IDs.TMDB))}
		if inJsonDb, err := InJsonDB(strconv.Itoa(show.IDs.TMDB), LShow); err == nil && inJsonDb == true {
			libraryAction = []string{"LOCALIZE[30253]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/library/show/remove/%d", show.IDs.TMDB))}
		}

		watchlistAction := []string{"LOCALIZE[30255]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/show/%d/watchlist/add", show.IDs.TMDB))}
		if InShowsWatchlist(show.IDs.TMDB) {
			watchlistAction = []string{"LOCALIZE[30256]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/show/%d/watchlist/remove", show.IDs.TMDB))}
		}

		collectionAction := []string{"LOCALIZE[30258]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/show/%d/collection/add", show.IDs.TMDB))}
		if InShowsCollection(show.IDs.TMDB) {
			collectionAction = []string{"LOCALIZE[30259]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/show/%d/collection/remove", show.IDs.TMDB))}
		}

		item.ContextMenu = [][]string{
			libraryAction,
			watchlistAction,
			collectionAction,
			[]string{"LOCALIZE[30035]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/setviewmode/tvshows"))},
		}
		items = append(items, item)
	}
	if page >= 0 && hasNextPage > 0 {
		path := ctx.Request.URL.Path
		nextpage := &xbmc.ListItem{
			Label: "LOCALIZE[30218]",
			Path: UrlForXBMC(fmt.Sprintf("%s?page=%d", path, page + 1)),
			Thumbnail: config.AddonResource("img", "nextpage.png"),
		}
		items = append(items, nextpage)
	}
	ctx.JSON(200, xbmc.NewView("tvshows", items))
}

func TraktPopularShows(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	shows, err := trakt.TopShows("popular", pageParam)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktShows(shows, ctx, page)
}

func TraktTrendingShows(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	shows, err := trakt.TopShows("trending", pageParam)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktShows(shows, ctx, page)
}

func TraktMostPlayedShows(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	shows, err := trakt.TopShows("played", pageParam)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktShows(shows, ctx, page)
}

func TraktMostWatchedShows(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	shows, err := trakt.TopShows("watched", pageParam)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktShows(shows, ctx, page)
}

func TraktMostCollectedShows(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	shows, err := trakt.TopShows("collected", pageParam)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktShows(shows, ctx, page)
}

func TraktMostAnticipatedShows(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	shows, err := trakt.TopShows("anticipated", pageParam)
	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
	}
	renderTraktShows(shows, ctx, page)
}
