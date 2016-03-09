package api

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/trakt"
	"github.com/scakemyer/quasar/xbmc"
)

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
		item.ContextMenu = [][]string{
			[]string{"LOCALIZE[30202]", fmt.Sprintf("XBMC.PlayMedia(%s)", movieLinksUrl)},
			[]string{"LOCALIZE[30023]", fmt.Sprintf("XBMC.PlayMedia(%s)", playUrl)},
			[]string{"LOCALIZE[30203]", "XBMC.Action(Info)"},
			libraryAction,
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
	renderTraktMovies(trakt.TopMovies("popular", pageParam), ctx, page)
}

func TraktTrendingMovies(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	renderTraktMovies(trakt.TopMovies("trending", pageParam), ctx, page)
}

func TraktMostPlayedMovies(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	renderTraktMovies(trakt.TopMovies("played", pageParam), ctx, page)
}

func TraktMostWatchedMovies(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	renderTraktMovies(trakt.TopMovies("watched", pageParam), ctx, page)
}

func TraktMostCollectedMovies(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	renderTraktMovies(trakt.TopMovies("collected", pageParam), ctx, page)
}

func TraktMostAnticipatedMovies(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	renderTraktMovies(trakt.TopMovies("anticipated", pageParam), ctx, page)
}

func TraktBoxOffice(ctx *gin.Context) {
	renderTraktMovies(trakt.TopMovies("boxoffice", "1"), ctx, 0)
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
		item.ContextMenu = [][]string{
			libraryAction,
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
	renderTraktShows(trakt.TopShows("popular", pageParam), ctx, page)
}

func TraktTrendingShows(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	renderTraktShows(trakt.TopShows("trending", pageParam), ctx, page)
}

func TraktMostPlayedShows(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	renderTraktShows(trakt.TopShows("played", pageParam), ctx, page)
}

func TraktMostWatchedShows(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	renderTraktShows(trakt.TopShows("watched", pageParam), ctx, page)
}

func TraktMostCollectedShows(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	renderTraktShows(trakt.TopShows("collected", pageParam), ctx, page)
}

func TraktMostAnticipatedShows(ctx *gin.Context) {
	pageParam := ctx.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageParam)
	renderTraktShows(trakt.TopShows("anticipated", pageParam), ctx, page)
}
