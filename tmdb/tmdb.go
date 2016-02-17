package tmdb

import (
	"fmt"
	"sync"
	"time"
	"path"
	"errors"
	"strconv"
	"math/rand"

	"github.com/op/go-logging"
	"github.com/jmcvetta/napping"
	"github.com/scakemyer/quasar/cache"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/util"
)

var (
	tmdbLog = logging.MustGetLogger("tmdb")
)

type IdName struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type Genre IdName

type GenreList struct {
	Genres []*Genre `json:"genres"`
}

type Shows []*Show
type SeasonList []*Season
type EpisodeList []*Episode

type Show struct {
	Entity

	EpisodeRunTime      []int        `json:"episode_run_time"`
	Genres              []*Genre     `json:"genres"`
	Homepage            string       `json:"homepage"`
	InProduction        bool         `json:"in_production"`
	FirstAirDate        string       `json:"first_air_date"`
	LastAirDate         string       `json:"last_air_date"`
	Networks            []*IdName    `json:"networks"`
	NumberOfEpisodes    int          `json:"number_of_episodes"`
	NumberOfSeasons     int          `json:"number_of_seasons"`
	OriginalName        string       `json:"original_name"`
	OriginCountry       []string     `json:"origin_country"`
	Overview            string       `json:"overview"`
	EpisodeRuntime      []int        `json:"runtime"`
	RawPopularity       interface{}  `json:"popularity"`
	Popularity          float64      `json:"-"`
	ProductionCompanies []*IdName    `json:"production_companies"`
	Status              string       `json:"status"`
	ExternalIDs         *ExternalIDs `json:"external_ids"`
	Translations        *struct {
		Translations []*Language `json:"translations"`
	} `json:"translations"`

	Credits *Credits `json:"credits,omitempty"`
	Images  *Images  `json:"images,omitempty"`

	Seasons SeasonList `json:"seasons"`
}

type Season struct {
	Id           int    `json:"id"`
	Name         string `json:"name,omitempty"`
	Season       int    `json:"season_number"`
	EpisodeCount int    `json:"episode_count,omitempty"`
	AirDate      string `json:"air_date"`
	Poster       string `json:"poster_path"`

	Episodes EpisodeList `json:"episodes"`

	// Images  *Images  `json:"images,omitempty"`
}

type Episode struct {
	Id            int     `json:"id"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	AirDate       string  `json:"air_date"`
	SeasonNumber  int     `json:"season_number"`
	EpisodeNumber int     `json:"episode_number"`
	VoteAverage   float32 `json:"vote_average"`
	StillPath     string  `json:"still_path"`

	// Images  *Images  `json:"images,omitempty"`
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
	FirstAirDate  string    `json:"first_air_date"`
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

type FindResult struct {
	MovieResults     []*Entity `json:"movie_results"`
	PersonResults    []*Entity `json:"person_results"`
	TVResults        []*Entity `json:"tv_results"`
	TVEpisodeResults []*Entity `json:"tv_episode_results"`
	TVSeasonResults  []*Entity `json:"tv_season_results"`
}

type List struct {
	CreatedBy     string    `json:"created_by"`
	Description   string    `json:"description"`
	FavoriteCount int       `json:"favorite_count"`
	Id            string    `json:"id"`
	ItemCount     int       `json:"item_count"`
	ISO_639_1     string    `json:"iso_639_1"`
	Name          string    `json:"name"`
	PosterPath    string    `json:"poster_path"`
	Items         []*Entity `json:"items"`
}

type Trailer struct {
	Name   string `json:"name"`
	Size   string `json:"size"`
	Source string `json:"source"`
	Type   string `json:"type"`
}

const (
	tmdbEndpoint            = "http://api.themoviedb.org/3/"
	imageEndpoint           = "http://image.tmdb.org/t/p/"
	burstRate               = 40
	burstTime               = 10 * time.Second
	simultaneousConnections = 20
	cacheTime               = 60 * 24 * time.Hour
)

var (
	apiKeys = []string{
		"ae4bd1b6fce2a5648671bfc171d15ba4",
		"8cf43ad9c085135b9479ad5cf6bbcbda",
	}
	apiKey = apiKeys[rand.Int()%len(apiKeys)]
)

var rateLimiter = util.NewRateLimiter(burstRate, burstTime, simultaneousConnections)

func CheckApiKey() {
	tmdbLog.Info("Checking TMDB API key...")

	customApiKey := config.Get().TMDBApiKey
	if customApiKey != "" {
		apiKeys = append(apiKeys, customApiKey)
		apiKey = customApiKey
	}

	result := false
	for index := len(apiKeys); index >= 0; index-- {
		result = tmdbCheck(apiKey)
		if result {
			tmdbLog.Noticef("TMDB API key check passed, using %s...", apiKey[:7])
			break
		} else {
			tmdbLog.Warningf("TMDB API key failed: %s", apiKey)
			if apiKey == apiKeys[index] {
				apiKeys = append(apiKeys[:index], apiKeys[index + 1:]...)
			}
			apiKey = apiKeys[rand.Int()%len(apiKeys)]
		}
	}
	if result == false {
		tmdbLog.Error("No valid TMDB API key found")
	}
}

func tmdbCheck(key string) bool {
	var result *Entity

	urlValues := napping.Params{
		"api_key": key,
	}.AsUrlValues()

	resp, err := napping.Get(
		tmdbEndpoint + "movie/550",
		&urlValues,
		&result,
		nil,
	)

	if err != nil {
		panic(err)
	}
	if resp.Status() != 200 {
		return false
	}

	return true
}

func imageURL(uri string, size string) string {
	return imageEndpoint + size + uri
}

func ListEntities(endpoint string, params napping.Params) []*Entity {
	var wg sync.WaitGroup
	entities := make([]*Entity, popularMoviesMaxPages * moviesPerPage)
	params["api_key"] = apiKey
	params["language"] = "en"

	wg.Add(popularMoviesMaxPages)
	for i := 0; i < popularMoviesMaxPages; i++ {
		go func(page int) {
			defer wg.Done()
			var tmp *EntityList
			tmpParams := napping.Params{
				"page": strconv.Itoa(popularMoviesStartPage + page),
			}
			for k, v := range params {
				tmpParams[k] = v
			}
      urlValues := tmpParams.AsUrlValues()
			rateLimiter.Call(func() {
				resp, err := napping.Get(
					tmdbEndpoint + endpoint,
					&urlValues,
					&tmp,
					nil,
				)
				if err != nil {
					panic(err)
				}
				if resp.Status() != 200 {
					panic(errors.New(fmt.Sprintf("Bad status: %d", resp.Status())))
				}
			})
			for i, entity := range tmp.Results {
				entities[page * moviesPerPage + i] = entity
			}
		}(i)
	}
	wg.Wait()

	return entities
}

func Find(externalId string, externalSource string) *FindResult {
	var result *FindResult

	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := fmt.Sprintf("com.tmdb.find.%s.%s", externalSource, externalId)
	if err := cacheStore.Get(key, &result); err != nil {
		rateLimiter.Call(func() {
      urlValues := napping.Params{
				"api_key": apiKey,
				"external_source": externalSource,
			}.AsUrlValues()
			resp, err := napping.Get(
				tmdbEndpoint + "find/" + externalId,
				&urlValues,
				&result,
				nil,
			)
			if err != nil {
				panic(err)
			}
			if resp.Status() != 200 {
				panic(errors.New(fmt.Sprintf("Bad status: %d", resp.Status())))
			}
			cacheStore.Set(key, result, 365 * 24 * time.Hour)
		})
	}

	return result
}
