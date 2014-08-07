package tmdb

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jmcvetta/napping"
	"github.com/steeve/pulsar/xbmc"
)

const (
	moviesPerPage         = 20
	popularMoviesMaxPages = 10
)

type Movie struct {
	Entity

	IMDBId              string      `json:"imdb_id"`
	Overview            string      `json:"overview"`
	ProductionCompanies []*IdName   `json:"production_companies"`
	Runtime             int         `json:"runtime"`
	TagLine             string      `json:"tagline"`
	RawPopularity       interface{} `json:"popularity"`
	Popularity          float64     `json:"-"`

	Credits *Credits `json:"credits,omitempty"`
	Images  *Images  `json:"images,omitempty"`
}

type Movies []*Movie

func GetMovieFromIMDB(imdbId string) *Movie {
	return getMovieById(imdbId)
}

func GetMovie(tmdbId int) *Movie {
	return getMovieById(strconv.Itoa(tmdbId))
}

func getMovieById(movieId string) *Movie {
	movie := Movie{}
	rateLimiter.Call(func() {
		napping.Get(
			endpoint+"movie/"+movieId,
			&napping.Params{"api_key": apiKey, "append_to_response": "credits,images"},
			&movie,
			nil,
		)
	})
	switch t := movie.RawPopularity.(type) {
	case string:
		popularity, _ := strconv.ParseFloat(t, 64)
		movie.Popularity = popularity
	case float64:
		movie.Popularity = t
	}
	return &movie
}

func GetMovies(tmdbIds []int) Movies {
	var wg sync.WaitGroup
	movies := make(Movies, len(tmdbIds))
	wg.Add(len(tmdbIds))
	for i, tmdbId := range tmdbIds {
		go func(i int, tmdbId int) {
			defer wg.Done()
			movies[i] = GetMovie(tmdbId)
		}(i, tmdbId)
	}
	wg.Wait()
	return movies
}

func GetMovieGenres() []*Genre {
	genres := GenreList{}
	rateLimiter.Call(func() {
		napping.Get(
			endpoint+"genre/list",
			&napping.Params{"api_key": apiKey},
			&genres,
			nil,
		)
	})
	return genres.Genres
}

func SearchMovies(query string) Movies {
	var results EntityList
	rateLimiter.Call(func() {
		_, err := napping.Get(
			endpoint+"search/movie",
			&napping.Params{
				"api_key": apiKey,
				"query":   query,
			},
			&results,
			nil,
		)
		fmt.Println(err)
	})
	fmt.Println(results)
	tmdbIds := make([]int, 0, len(results.Results))
	for _, movie := range results.Results {
		tmdbIds = append(tmdbIds, movie.Id)
	}
	return GetMovies(tmdbIds)
}

type ByPopularity Movies

func (a ByPopularity) Len() int           { return len(a) }
func (a ByPopularity) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByPopularity) Less(i, j int) bool { return a[i].Popularity < a[j].Popularity }

func PopularMovies(genre string) []*Entity {
	var wg sync.WaitGroup
	movies := make([]*Entity, popularMoviesMaxPages*moviesPerPage)

	wg.Add(popularMoviesMaxPages)
	for i := 1; i < popularMoviesMaxPages; i++ {
		go func(page int) {
			defer wg.Done()
			var tmp EntityList
			rateLimiter.Call(func() {
				napping.Get(
					endpoint+"discover/movie",
					&napping.Params{
						"api_key":     apiKey,
						"sort_by":     "popularity.desc",
						"language":    "en",
						"page":        strconv.Itoa(page),
						"with_genres": genre,
					},
					&tmp,
					nil,
				)
			})
			for i, movie := range tmp.Results {
				movies[page*moviesPerPage+i] = movie
			}
		}(i)
	}
	wg.Wait()

	return movies
}

func PopularMoviesComplete(genre string) Movies {
	moviesChan := make(chan *Movie)

	wg := sync.WaitGroup{}
	go func() {
		wg.Wait()
		close(moviesChan)
	}()
	for i := 1; i < popularMoviesMaxPages; i++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			tmp := EntityList{}
			rateLimiter.Call(func() {
				napping.Get(
					endpoint+"discover/movie",
					&napping.Params{
						"api_key":     apiKey,
						"sort_by":     "popularity.desc",
						"language":    "en",
						"page":        strconv.Itoa(page),
						"with_genres": genre,
					},
					&tmp,
					nil,
				)
			})
			for _, movie := range tmp.Results {
				moviesChan <- GetMovie(movie.Id)
			}
		}(i)
	}

	popularMovies := make(Movies, 0)
	for movie := range moviesChan {
		popularMovies = append(popularMovies, movie)
	}

	sort.Sort(sort.Reverse(ByPopularity(popularMovies)))

	return popularMovies
}

func (movie *Movie) ToListItem() *xbmc.ListItem {
	item := &xbmc.ListItem{
		Label: movie.OriginalTitle,
		Info: &xbmc.ListItemInfo{
			Count:         movie.IMDBId,
			Title:         movie.OriginalTitle,
			OriginalTitle: movie.Title,
			Plot:          movie.Overview,
			PlotOutline:   movie.Overview,
			TagLine:       movie.TagLine,
			Duration:      movie.Runtime,
			Code:          movie.IMDBId,
		},
		Art: &xbmc.ListItemArt{},
	}
	genres := make([]string, 0, len(movie.Genres))
	for _, genre := range movie.Genres {
		genres = append(genres, genre.Name)
	}
	item.Info.Genre = strings.Join(genres, " / ")

	if len(movie.ProductionCompanies) > 0 {
		item.Info.Studio = movie.ProductionCompanies[0].Name
	}
	if movie.Credits != nil {
		item.Info.CastAndRole = make([]string, 0, len(movie.Credits.Cast))
		for _, cast := range movie.Credits.Cast {
			item.Info.CastAndRole = append(item.Info.CastAndRole, cast.Name+"|"+cast.Character)
		}
		directors := make([]string, 0)
		writers := make([]string, 0)
		for _, crew := range movie.Credits.Crew {
			switch crew.Job {
			case "Director":
				directors = append(directors, crew.Name)
			case "Writer":
				writers = append(writers, crew.Name)
			}
		}
		item.Info.Director = strings.Join(directors, " / ")
		item.Info.Writer = strings.Join(writers, " / ")
	}
	if movie.Images != nil {
		if len(movie.Images.Posters) > 0 {
			item.Art.Poster = imageURL(movie.Images.Posters[0].FilePath, "w500")
		}
		if len(movie.Images.Backdrops) > 0 {
			item.Art.FanArt = imageURL(movie.Images.Backdrops[0].FilePath, "original")
		}
	}
	return item
}
