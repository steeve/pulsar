package tmdb

import (
	"path"
	"strconv"
	"sync"

	"github.com/jmcvetta/napping"
	"github.com/steeve/pulsar/cache"
	"github.com/steeve/pulsar/config"
)

type Show struct {
	Entity

	EpisodeRunTime   []int       `json:"episode_run_time"`
	Genres           []*Genre    `json:"genres"`
	Homepage         string      `json:"homepage"`
	InProduction     bool        `json:"in_production"`
	LastAirDate      string      `json:"last_air_date"`
	Networks         []*IdName   `json:"networks"`
	NumberOfEpisodes int         `json:"number_of_episodes"`
	NumberOfSeasons  int         `json:"number_of_seasons"`
	OriginalName     string      `json:"original_name"`
	OriginCountry    string      `json:"origin_country"`
	Overview         string      `json:"overview"`
	EpisodeRuntime   []int       `json:"runtime"`
	RawPopularity    interface{} `json:"popularity"`
	Popularity       float64     `json:"-"`
	Status           string      `json:"status"`

	Credits *Credits `json:"credits,omitempty"`
	Images  *Images  `json:"images,omitempty"`
}

type Shows []*Show

func GetShow(showId int) *Show {
	var show Show
	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := "com.tmdb.movie." + strconv.Itoa(showId)
	if err := cacheStore.Get(key, &show); err != nil {
		rateLimiter.Call(func() {
			napping.Get(
				tmdbEndpoint+"tv/"+strconv.Itoa(showId),
				&napping.Params{"api_key": apiKey, "append_to_response": "credits,images"},
				&show,
				nil,
			)
		})
		cacheStore.Set(key, show, cacheTime)
	}
	switch t := show.RawPopularity.(type) {
	case string:
		if popularity, err := strconv.ParseFloat(t, 64); err == nil {
			show.Popularity = popularity
		}
	case float64:
		show.Popularity = t
	}
	return &show
}

func GetShows(showIds []int) Shows {
	var wg sync.WaitGroup
	shows := make(Shows, len(showIds))
	wg.Add(len(showIds))
	for i, showId := range showIds {
		go func(i int, showId int) {
			defer wg.Done()
			shows[i] = GetShow(showId)
		}(i, showId)
	}
	wg.Wait()
	return shows
}
