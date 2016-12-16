package trakt

import (
	"fmt"
	"errors"
	"strconv"
	"strings"
	"math/rand"

	"github.com/jmcvetta/napping"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/tmdb"
	"github.com/scakemyer/quasar/xbmc"
)

// Fill fanart from TMDB
func setFanart(movie *Movie) *Movie {
	tmdbImages := tmdb.GetImages(movie.IDs.TMDB)
	if tmdbImages == nil {
		return movie
	}
	if len(tmdbImages.Posters) > 0 {
		movie.Images.Poster.Full = tmdb.ImageURL(tmdbImages.Posters[0].FilePath, "w500")
		movie.Images.Thumbnail.Full = tmdb.ImageURL(tmdbImages.Posters[0].FilePath, "w500")
	}
	if len(tmdbImages.Backdrops) > 0 {
		movie.Images.FanArt.Full = tmdb.ImageURL(tmdbImages.Backdrops[0].FilePath, "w1280")
		movie.Images.Banner.Full = tmdb.ImageURL(tmdbImages.Backdrops[0].FilePath, "w1280")
	}
	return movie
}

func setFanarts(movies []*Movies) []*Movies {
	for i, movie := range movies {
		movies[i].Movie = setFanart(movie.Movie)
	}
	return movies
}

func GetMovie(Id string) (movie *Movie) {
	endPoint := fmt.Sprintf("movies/%s", Id)

	params := napping.Params{
		"extended": "full,images",
	}.AsUrlValues()

	resp, err := Get(endPoint, params)

	if err != nil {
		log.Error(err.Error())
		xbmc.Notify("Quasar", "GetMovie failed, check your logs.", config.AddonIcon())
	}

	resp.Unmarshal(&movie)
	movie = setFanart(movie)

	return movie
}

func SearchMovies(query string, page string) (movies []*Movies, err error) {
	endPoint := "search"

	params := napping.Params{
		"page": page,
		"limit": strconv.Itoa(config.Get().ResultsPerPage),
		"query": query,
		"extended": "full,images",
	}.AsUrlValues()

	resp, err := Get(endPoint, params)

	if err != nil {
		return movies, err
	} else if resp.Status() != 200 {
		return movies, errors.New(fmt.Sprintf("SearchMovies bad status: %d", resp.Status()))
	}

  // TODO use response headers for pagination limits:
  // X-Pagination-Page-Count:10
  // X-Pagination-Item-Count:100

	resp.Unmarshal(&movies)
	movies = setFanarts(movies)

	return movies, err
}

func TopMovies(topCategory string, page string) (movies []*Movies, err error) {
	endPoint := "movies/" + topCategory

	params := napping.Params{
		"page": page,
		"limit": strconv.Itoa(config.Get().ResultsPerPage),
		"extended": "full,images",
	}.AsUrlValues()

	resp, err := Get(endPoint, params)

	if err != nil {
		return movies, err
	} else if resp.Status() != 200 {
		return movies, errors.New(fmt.Sprintf("TopMovies bad status: %d", resp.Status()))
	}

	if topCategory == "popular" {
		var movieList []*Movie
		resp.Unmarshal(&movieList)

	  movieListing := make([]*Movies, 0)
	  for _, movie := range movieList {
			movieItem := Movies{
	      Movie: movie,
	    }
	    movieListing = append(movieListing, &movieItem)
	  }
		movies = movieListing
	} else {
		resp.Unmarshal(&movies)
	}

	movies = setFanarts(movies)

	return movies, err
}

func WatchlistMovies() (movies []*Movies, err error) {
	if err := Authorized(); err != nil {
		return movies, err
	}

	endPoint := "sync/watchlist/movies"

	params := napping.Params{
		"extended": "full,images",
	}.AsUrlValues()

	resp, err := GetWithAuth(endPoint, params)

	if err != nil {
		return movies, err
	} else if resp.Status() != 200 {
		return movies, errors.New(fmt.Sprintf("WatchlistMovies bad status: %d", resp.Status()))
	}

	var watchlist []*WatchlistMovie
	resp.Unmarshal(&watchlist)

	movieListing := make([]*Movies, 0)
	for _, movie := range watchlist {
		movieItem := Movies{
			Movie: movie.Movie,
		}
		movieListing = append(movieListing, &movieItem)
	}
	movies = movieListing

	movies = setFanarts(movies)

	return movies, err
}

func CollectionMovies() (movies []*Movies, err error) {
	if err := Authorized(); err != nil {
		return movies, err
	}

	endPoint := "sync/collection/movies"

	params := napping.Params{
		"extended": "full,images",
	}.AsUrlValues()

	resp, err := GetWithAuth(endPoint, params)

	if err != nil {
		return movies, err
	} else if resp.Status() != 200 {
		return movies, errors.New(fmt.Sprintf("CollectionMovies bad status: %d", resp.Status()))
	}

	var collection []*CollectionMovie
	resp.Unmarshal(&collection)

	movieListing := make([]*Movies, 0)
	for _, movie := range collection {
		movieItem := Movies{
			Movie: movie.Movie,
		}
		movieListing = append(movieListing, &movieItem)
	}
	movies = movieListing

	movies = setFanarts(movies)

	return movies, err
}

func Userlists() (lists []*List) {
	endPoint := fmt.Sprintf("users/%s/lists", config.Get().TraktUsername)

	params := napping.Params{}.AsUrlValues()

	var resp *napping.Response
	var err error

	if erra := Authorized(); erra != nil {
		resp, err = Get(endPoint, params)
	} else {
		resp, err = GetWithAuth(endPoint, params)
	}

	if err != nil || resp.Status() != 200 {
		return lists
	}

	resp.Unmarshal(&lists)

	return lists
}

func ListItemsMovies(listId string, page string) (movies []*Movies, err error) {
	endPoint := fmt.Sprintf("users/%s/lists/%s/items/movies", config.Get().TraktUsername, listId)

	params := napping.Params{}.AsUrlValues()

	if page != "0" {
		params = napping.Params{
			"page": page,
			"limit": strconv.Itoa(config.Get().ResultsPerPage),
			"extended": "full,images",
		}.AsUrlValues()
	}

	var resp *napping.Response

	if erra := Authorized(); erra != nil {
		resp, err = Get(endPoint, params)
	} else {
		resp, err = GetWithAuth(endPoint, params)
	}

	if err != nil || resp.Status() != 200 {
		return movies, err
	}

	var list []*ListItem
	err = resp.Unmarshal(&list)

	movieListing := make([]*Movies, 0)
	for _, movie := range list {
		if movie.Movie == nil {
			continue
		}
		movieItem := Movies{
			Movie: movie.Movie,
		}
		movieListing = append(movieListing, &movieItem)
	}
	movies = movieListing

	movies = setFanarts(movies)

	return movies, err
}

func (movie *Movie) ToListItem() *xbmc.ListItem {
	return &xbmc.ListItem{
		Label: movie.Title,
		Info: &xbmc.ListItemInfo{
			Count:         rand.Int(),
			Title:         movie.Title,
			OriginalTitle: movie.Title,
			Year:          movie.Year,
			Genre:         strings.Title(strings.Join(movie.Genres, " / ")),
			Plot:          movie.Overview,
			PlotOutline:   movie.Overview,
			TagLine:       movie.TagLine,
			Rating:        movie.Rating,
			Votes:         strconv.Itoa(movie.Votes),
			Duration:      movie.Runtime * 60,
			MPAA:          movie.Certification,
			Code:          movie.IDs.IMDB,
			IMDBNumber:    movie.IDs.IMDB,
			Trailer:       movie.Trailer,
			DBTYPE:        "movie",
			Mediatype:     "movie",
		},
		Art: &xbmc.ListItemArt{
			Poster: movie.Images.Poster.Full,
			FanArt: movie.Images.FanArt.Full,
			Banner: movie.Images.Banner.Full,
			Thumbnail: movie.Images.Thumbnail.Full,
		},
	}
}
