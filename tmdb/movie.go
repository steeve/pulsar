package tmdb

import (
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/jmcvetta/napping"
	"github.com/steeve/pulsar/cache"
	"github.com/steeve/pulsar/config"
	"github.com/steeve/pulsar/xbmc"
)

const (
	moviesPerPage          = 20
	popularMoviesMaxPages  = 20
	popularMoviesStartPage = 1
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
	AlternativeTitles   *struct {
		Titles []*AlternativeTitle `json:"titles"`
	} `json:"alternative_titles"`
	SpokenLanguages []*struct {
		ISO_639_1 string `json:"iso_639_1"`
		Name      string `json:"name"`
	} `json:"spoken_languages"`

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
	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := "com.tmdb.movie." + movieId
	if err := cacheStore.Get(key, &movie); err != nil {
		rateLimiter.Call(func() {
			napping.Get(
				tmdbEndpoint+"movie/"+movieId,
				&napping.Params{"api_key": apiKey, "append_to_response": "credits,images,alternative_titles"},
				&movie,
				nil,
			)
			cacheStore.Set(key, movie, cacheTime)
		})
	}
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
			tmdbEndpoint+"genre/list",
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
		napping.Get(
			tmdbEndpoint+"search/movie",
			&napping.Params{
				"api_key": apiKey,
				"query":   query,
			},
			&results,
			nil,
		)
	})
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

func ListMovies(endpoint string, genre string, sortBy string) []*Entity {
	var wg sync.WaitGroup
	movies := make([]*Entity, popularMoviesMaxPages*moviesPerPage)

	wg.Add(popularMoviesMaxPages)
	for i := 0; i < popularMoviesMaxPages; i++ {
		go func(page int) {
			defer wg.Done()
			var tmp EntityList
			rateLimiter.Call(func() {
				napping.Get(
					tmdbEndpoint+endpoint,
					&napping.Params{
						"api_key":     apiKey,
						"sort_by":     sortBy,
						"language":    "en",
						"page":        strconv.Itoa(popularMoviesStartPage + page),
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

func ListMoviesComplete(endpoint string, genre string, sortBy string) Movies {
	movies := make(Movies, popularMoviesMaxPages*moviesPerPage)

	wg := sync.WaitGroup{}
	for i := 0; i < popularMoviesMaxPages; i++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			tmp := EntityList{}
			rateLimiter.Call(func() {
				napping.Get(
					tmdbEndpoint+endpoint,
					&napping.Params{
						"api_key":     apiKey,
						"sort_by":     sortBy,
						"language":    "en",
						"page":        strconv.Itoa(popularMoviesStartPage + page),
						"with_genres": genre,
					},
					&tmp,
					nil,
				)
			})
			for i, movie := range tmp.Results {
				movies[page*moviesPerPage+i] = GetMovie(movie.Id)
			}
		}(i)
	}
	wg.Wait()

	return movies
}

func PopularMoviesComplete(genre string) Movies {
	return ListMoviesComplete("movie/popular", genre, "")
}

func TopRatedMoviesComplete(genre string) Movies {
	return ListMoviesComplete("movie/top_rated", genre, "")
}

func (movie *Movie) ToListItem() *xbmc.ListItem {
	year, _ := strconv.Atoi(strings.Split(movie.ReleaseDate, "-")[0])

	item := &xbmc.ListItem{
		Label: movie.OriginalTitle,
		Info: &xbmc.ListItemInfo{
			Year:          year,
			Count:         movie.IMDBId,
			Title:         movie.OriginalTitle,
			OriginalTitle: movie.Title,
			Plot:          movie.Overview,
			PlotOutline:   movie.Overview,
			TagLine:       movie.TagLine,
			Duration:      movie.Runtime,
			Code:          movie.IMDBId,
			Date:          movie.ReleaseDate,
			Votes:         strconv.Itoa(movie.VoteCount),
			Rating:        movie.VoteAverage,
		},
		Art: &xbmc.ListItemArt{},
	}
	genres := make([]string, 0, len(movie.Genres))
	for _, genre := range movie.Genres {
		genres = append(genres, genre.Name)
	}
	item.Info.Genre = strings.Join(genres, " / ")

	if len(movie.SpokenLanguages) > 0 {
		item.StreamInfo = &xbmc.StreamInfo{
			Audio: &xbmc.StreamInfoEntry{
				Language: movie.SpokenLanguages[0].ISO_639_1,
			},
		}
	}

	if len(movie.ProductionCompanies) > 0 {
		item.Info.Studio = movie.ProductionCompanies[0].Name
	}
	if movie.Credits != nil {
		item.Info.CastAndRole = make([][]string, 0)
		for _, cast := range movie.Credits.Cast {
			item.Info.CastAndRole = append(item.Info.CastAndRole, []string{cast.Name, cast.Character})
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
