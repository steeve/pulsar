package trakt

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/jmcvetta/napping"
	"github.com/scakemyer/quasar/xbmc"
)

type MovieList []*Movie

type Movie struct {
	Title         string      `json:"title"`
	Year          int         `json:"year"`
	Released      int         `json:"released"`
	URL           string      `json:"url"`
	Trailer       string      `json:"trailer"`
	Runtime       int         `json:"runtime"`
	TagLine       string      `json:"tagline"`
	Overview      string      `json:"overview"`
	Certification string      `json:"certification"`
	IMDBId        string      `json:"imdb_id"`
	TMDBId        interface{} `json:"tmdb_id"`
	Poster        string      `json:"poster"`
	Images        *Images     `json:"images"`
	Ratings       *Ratings    `json:"ratings"`
	Genres        []string    `json:"genres"`
}

func SearchMovies(query string) MovieList {
	var movies MovieList
	napping.Get(fmt.Sprintf("%s/search/movies.json/%s", ENDPOINT, APIKEY),
		&napping.Params{"query": query, "limit": SearchLimit},
		&movies,
		nil)
	return movies
}

func TrendingMovies() (movies MovieList) {
	napping.Get(fmt.Sprintf("%s/movies/trending.json/%s", ENDPOINT, APIKEY), nil, &movies, nil)
	return
}

func NewMovie(IMDBId string) (movie *Movie) {
	napping.Get(fmt.Sprintf("%s/movie/summary.json/%s/%s", ENDPOINT, APIKEY, IMDBId), nil, &movie, nil)
	return
}

func (movie *Movie) ToListItem() *xbmc.ListItem {
	return &xbmc.ListItem{
		Label: movie.Title,
		Info: &xbmc.ListItemInfo{
			Count:       rand.Int(),
			Title:       movie.Title,
			Genre:       strings.Join(movie.Genres, " / "),
			Plot:        movie.Overview,
			PlotOutline: movie.Overview,
			TagLine:     movie.TagLine,
			Rating:      float32(movie.Ratings.Percentage) / 10,
			Duration:    movie.Runtime,
			Code:        movie.IMDBId,
			Trailer:     movie.Trailer,
		},
		Art: &xbmc.ListItemArt{
			Poster: movie.Images.Poster,
			FanArt: movie.Images.FanArt,
		},
	}
}
