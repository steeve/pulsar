package trakt

import (
	"fmt"
	"errors"
	"strconv"
	"strings"
	"math/rand"

	"github.com/jmcvetta/napping"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/xbmc"
)

func GetMovie(Id string) (movie *Movie) {
	endPoint := fmt.Sprintf("movies/%s", Id)

	params := napping.Params{
		"extended": "full,images",
	}.AsUrlValues()

	resp, err := Get(endPoint, params)

	if err != nil {
		panic(err)
	}

	resp.Unmarshal(&movie)
	return movie
}

func SearchMovies(query string, page string) (movies []*Movies) {
	endPoint := "search"

	params := napping.Params{
		"page": page,
		"limit": Limit,
		"query": query,
		"extended": "full,images",
	}.AsUrlValues()

	resp, err := Get(endPoint, params)

	if err != nil {
		panic(err)
	}
	if resp.Status() != 200 {
		panic(errors.New(fmt.Sprintf("Bad status: %d", resp.Status())))
	}

  // TODO use response headers for pagination limits:
  // X-Pagination-Page-Count:10
  // X-Pagination-Item-Count:100

	resp.Unmarshal(&movies)
	return movies
}

func TopMovies(topCategory string, page string) (movies []*Movies) {
	endPoint := "movies/" + topCategory

	params := napping.Params{
		"page": page,
		"limit": Limit,
		"extended": "full,images",
	}.AsUrlValues()

	resp, err := Get(endPoint, params)

	if err != nil {
		panic(err)
	}
	if resp.Status() != 200 {
		panic(errors.New(fmt.Sprintf("Bad status: %d", resp.Status())))
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
	return movies
}

func WatchlistMovies() (movies []*Movies) {
	if config.Get().TraktToken == "" {
		err := Authorize()
		if err != nil {
			return movies
		}
	}

	endPoint := "sync/watchlist/movies"

	params := napping.Params{
		"extended": "full,images",
	}.AsUrlValues()

	resp, err := GetWithAuth(endPoint, params)

	if err != nil {
		panic(err)
	}
	if resp.Status() != 200 {
		panic(errors.New(fmt.Sprintf("Bad status: %d", resp.Status())))
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

	return movies
}

func (movie *Movie) ToListItem() *xbmc.ListItem {
	return &xbmc.ListItem{
		Label: movie.Title,
		Info: &xbmc.ListItemInfo{
			Count:       rand.Int(),
			Title:       movie.Title,
			Year:        movie.Year,
			Genre:       strings.Title(strings.Join(movie.Genres, " / ")),
			Plot:        movie.Overview,
			PlotOutline: movie.Overview,
			TagLine:     movie.TagLine,
			Rating:      movie.Rating,
			Votes:       strconv.Itoa(movie.Votes),
			Duration:    movie.Runtime * 60,
			MPAA:        movie.Certification,
			Code:        movie.IDs.IMDB,
			Trailer:     movie.Trailer,
		},
		Art: &xbmc.ListItemArt{
			Poster: movie.Images.Poster.Full,
			FanArt: movie.Images.FanArt.Full,
			Banner: movie.Images.Banner.Full,
			Thumbnail: movie.Images.Thumbnail.Full,
		},
	}
}
