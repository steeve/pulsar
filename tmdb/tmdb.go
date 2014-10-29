package tmdb

import (
	"strconv"
	"sync"
	"time"

	"github.com/jmcvetta/napping"
	"github.com/steeve/pulsar/util"
)

type IdName struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type Genre IdName

type GenreList struct {
	Genres []*Genre `json:"genres"`
}

type Image struct {
	FilePath  string `json:"file_path"`
	Height    int    `json:"height"`
	ISO_639_1 string `json:"iso_639_1"`
	Width     int    `json:"width"`
}

type Images struct {
	Backdrops []*Image `json:"backdrops"`
	Posters   []*Image `json:"posters"`
	Stills    []*Image `json:"stills"`
}

type Cast struct {
	IdName
	CastId      int    `json:"cast_id"`
	Character   string `json:"character"`
	CreditId    string `json:"credit_id"`
	Order       int    `json:"order"`
	ProfilePath string `json:"profile_path"`
}

type Crew struct {
	IdName
	CreditId    string `json:"credit_id"`
	Department  string `json:"department"`
	Job         string `json:"job"`
	ProfilePath string `json:"profile_path"`
}

type Credits struct {
	Cast []*Cast `json:"cast"`
	Crew []*Crew `json:"crew"`
}

type Entity struct {
	IsAdult       bool      `json:"adult"`
	BackdropPath  string    `json:"backdrop_path"`
	Id            int       `json:"id"`
	Genres        []*IdName `json:"genres"`
	OriginalTitle string    `json:"original_title,omitempty"`
	ReleaseDate   string    `json:"release_date"`
	PosterPath    string    `json:"poster_path"`
	Title         string    `json:"title,omitempty"`
	VoteAverage   float32   `json:"vote_average"`
	VoteCount     int       `json:"vote_count"`
	OriginalName  string    `json:"original_name,omitempty"`
	Name          string    `json:"name,omitempty"`
}

type EntityList struct {
	Page         int       `json:"page"`
	Results      []*Entity `json:"results"`
	TotalPages   int       `json:"total_pages"`
	TotalResults int       `json:"total_results"`
}

type ExternalIDs struct {
	IMDBId      string `json:"imdb_id"`
	FreeBaseID  string `json:"freebase_id"`
	FreeBaseMID string `json:"freebase_mid"`
	TVDBID      int    `json:"tvdb_id"`
	TVRageID    int    `json:"tvrage_id"`
}

type AlternativeTitle struct {
	ISO_3166_1 string `json:"iso_3166_1"`
	Title      string `json:"title"`
}

type Language struct {
	ISO_639_1   string `json:"iso_639_1"`
	Name        string `json:"name"`
	EnglishName string `json:"english_name,omitempty"`
}

const (
	tmdbEndpoint            = "http://api.themoviedb.org/3/"
	imageEndpoint           = "http://image.tmdb.org/t/p/"
	apiKey                  = "57983e31fb435df4df77afb854740ea9"
	burstRate               = 30
	burstTime               = 1 * time.Second
	simultaneousConnections = 20
	cacheTime               = 60 * 24 * time.Hour
)

var rateLimiter = util.NewRateLimiter(burstRate, burstTime, simultaneousConnections)

func imageURL(uri string, size string) string {
	return imageEndpoint + size + uri
}

func ListEntities(endpoint string, genre string, sortBy string) []*Entity {
	var wg sync.WaitGroup
	entities := make([]*Entity, popularMoviesMaxPages*moviesPerPage)

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
			for i, entity := range tmp.Results {
				entities[page*moviesPerPage+i] = entity
			}
		}(i)
	}
	wg.Wait()

	return entities
}
