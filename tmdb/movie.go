package tmdb

import (
	"fmt"
	"math/rand"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

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
	SpokenLanguages []*Language  `json:"spoken_languages"`
	ExternalIDs     *ExternalIDs `json:"external_ids"`

	Translations *struct {
		Translations []*Language `json:"translations"`
	} `json:"translations"`

	Trailers *struct {
		Youtube []*Trailer `json:"youtube"`
	} `json:"trailers"`

	Credits *Credits `json:"credits,omitempty"`
	Images  *Images  `json:"images,omitempty"`
}

type Movies []*Movie

func GetMovieFromIMDB(imdbId string, language string) *Movie {
	return getMovieById(imdbId, language)
}

func GetMovie(tmdbId int, language string) *Movie {
	return getMovieById(strconv.Itoa(tmdbId), language)
}

func getMovieById(movieId string, language string) *Movie {
	var movie *Movie
	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := fmt.Sprintf("com.tmdb.movie.%s.%s", movieId, language)
	if err := cacheStore.Get(key, &movie); err != nil {
		rateLimiter.Call(func() {
			napping.Get(
				tmdbEndpoint+"movie/"+movieId,
				&napping.Params{"api_key": apiKey, "append_to_response": "credits,images,alternative_titles,translations,external_ids,trailers", "language": language},
				&movie,
				nil,
			)
			if movie != nil {
				cacheStore.Set(key, movie, cacheTime)
			}
		})
	}
	if movie == nil {
		return nil
	}
	switch t := movie.RawPopularity.(type) {
	case string:
		popularity, _ := strconv.ParseFloat(t, 64)
		movie.Popularity = popularity
	case float64:
		movie.Popularity = t
	}
	return movie
}

func GetMovies(tmdbIds []int, language string) Movies {
	var wg sync.WaitGroup
	movies := make(Movies, len(tmdbIds))
	wg.Add(len(tmdbIds))
	for i, tmdbId := range tmdbIds {
		go func(i int, tmdbId int) {
			defer wg.Done()
			movies[i] = GetMovie(tmdbId, language)
		}(i, tmdbId)
	}
	wg.Wait()
	return movies
}

func GetMovieGenres(language string) []*Genre {
	genres := GenreList{}
	rateLimiter.Call(func() {
		napping.Get(
			tmdbEndpoint+"genre/movie/list",
			&napping.Params{"api_key": apiKey, "language": language},
			&genres,
			nil,
		)
	})
	return genres.Genres
}

func SearchMovies(query string, language string) Movies {
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
	return GetMovies(tmdbIds, language)
}

func GetList(listId string, language string) Movies {
	var results *List
	rateLimiter.Call(func() {
		napping.Get(
			tmdbEndpoint+"list/"+listId,
			&napping.Params{
				"api_key": apiKey,
			},
			&results,
			nil,
		)
	})
	tmdbIds := make([]int, 0, len(results.Items))
	for _, movie := range results.Items {
		tmdbIds = append(tmdbIds, movie.Id)
	}
	return GetMovies(tmdbIds, language)
}

type ByPopularity Movies

func (a ByPopularity) Len() int           { return len(a) }
func (a ByPopularity) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByPopularity) Less(i, j int) bool { return a[i].Popularity < a[j].Popularity }

func ListMoviesComplete(endpoint string, params napping.Params) Movies {
	movies := make(Movies, popularMoviesMaxPages*moviesPerPage)
	params["api_key"] = apiKey

	wg := sync.WaitGroup{}
	for i := 0; i < popularMoviesMaxPages; i++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			var tmp *EntityList
			tmpParams := napping.Params{
				"page": strconv.Itoa(popularMoviesStartPage + page),
			}
			for k, v := range params {
				tmpParams[k] = v
			}
			rateLimiter.Call(func() {
				napping.Get(
					tmdbEndpoint+endpoint,
					&tmpParams,
					&tmp,
					nil,
				)
			})
			for i, movie := range tmp.Results {
				movies[page*moviesPerPage+i] = GetMovie(movie.Id, params["language"])
			}
		}(i)
	}
	wg.Wait()

	return movies
}

func PopularMoviesComplete(genre string, language string) Movies {
	return ListMoviesComplete("discover/movie", napping.Params{
		"language":                 language,
		"sort_by":                  "popularity.desc",
		"primary_release_date.lte": time.Now().UTC().Format("2006-01-02"),
	})
}

func TopRatedMoviesComplete(genre string, language string) Movies {
	return ListMoviesComplete("movie/top_rated", napping.Params{"language": language})
}

func MostVotedMoviesComplete(genre string, language string) Movies {
	return ListMoviesComplete("discover/movie", napping.Params{
		"language":                 language,
		"sort_by":                  "vote_count.desc",
		"primary_release_date.lte": time.Now().UTC().Format("2006-01-02"),
	})
}

func (movie *Movie) ToListItem() *xbmc.ListItem {
	year, _ := strconv.Atoi(strings.Split(movie.ReleaseDate, "-")[0])

	item := &xbmc.ListItem{
		Label: movie.OriginalTitle,
		Info: &xbmc.ListItemInfo{
			Year:          year,
			Count:         rand.Int(),
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
		Art: &xbmc.ListItemArt{
			FanArt: imageURL(movie.BackdropPath, "w1280"),
			Poster: imageURL(movie.PosterPath, "w500"),
		},
	}
	item.Thumbnail = item.Art.Poster
	item.Art.Thumbnail = item.Art.Poster
	genres := make([]string, 0, len(movie.Genres))
	for _, genre := range movie.Genres {
		genres = append(genres, genre.Name)
	}
	item.Info.Genre = strings.Join(genres, " / ")

	if movie.Trailers != nil {
		for _, trailer := range movie.Trailers.Youtube {
			item.Info.Trailer = trailer.Source
			break
		}
	}

	for _, language := range movie.SpokenLanguages {
		item.StreamInfo = &xbmc.StreamInfo{
			Audio: &xbmc.StreamInfoEntry{
				Language: language.ISO_639_1,
			},
		}
		break
	}

	for _, company := range movie.ProductionCompanies {
		item.Info.Studio = company.Name
		break
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
	return item
}
