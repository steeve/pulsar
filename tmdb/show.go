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
	"github.com/i96751414/pulsar/cache"
	"github.com/i96751414/pulsar/config"
	"github.com/i96751414/pulsar/xbmc"
)

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
}

type Shows []*Show

func GetShow(showId int, language string) *Show {
	var show *Show
	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := fmt.Sprintf("com.tmdb.show.%d.%s", showId, language)
	if err := cacheStore.Get(key, &show); err != nil {
		rateLimiter.Call(func() {
            p := napping.Params{"api_key": apiKey, "append_to_response": "credits,images,alternative_titles,translations,external_ids", "language": language}.AsUrlValues()
			napping.Get(
				tmdbEndpoint+"tv/"+strconv.Itoa(showId),
				&p,
				&show,
				nil,
			)
		})
		if show != nil {
			cacheStore.Set(key, show, cacheTime)
		}
	}
	if show == nil {
		return nil
	}
	switch t := show.RawPopularity.(type) {
	case string:
		if popularity, err := strconv.ParseFloat(t, 64); err == nil {
			show.Popularity = popularity
		}
	case float64:
		show.Popularity = t
	}
	return show
}

func GetShows(showIds []int, language string) Shows {
	var wg sync.WaitGroup
	shows := make(Shows, len(showIds))
	wg.Add(len(showIds))
	for i, showId := range showIds {
		go func(i int, showId int) {
			defer wg.Done()
			shows[i] = GetShow(showId, language)
		}(i, showId)
	}
	wg.Wait()
	return shows
}

func SearchShows(query string, language string) Shows {
	var results EntityList
	rateLimiter.Call(func() {
        p := napping.Params{"api_key": apiKey,"query":query}.AsUrlValues()
		napping.Get(
			tmdbEndpoint+"search/tv",
			&p,
			&results,
			nil,
		)
	})
	tmdbIds := make([]int, 0, len(results.Results))
	for _, entity := range results.Results {
		tmdbIds = append(tmdbIds, entity.Id)
	}
	return GetShows(tmdbIds, language)
}

func ListShowsComplete(endpoint string, params napping.Params) Shows {
	shows := make(Shows, popularMoviesMaxPages*moviesPerPage)

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
            p := tmpParams.AsUrlValues()
			rateLimiter.Call(func() {
				napping.Get(
					tmdbEndpoint+endpoint,
					&p,
					&tmp,
					nil,
				)
			})
			for i, entity := range tmp.Results {
				shows[page*moviesPerPage+i] = GetShow(entity.Id, params["language"])
			}
		}(i)
	}
	wg.Wait()

	return shows
}

func PopularShowsComplete(genre string, language string) Shows {
	var p napping.Params
	if genre == "" {
		p = napping.Params{
			"language":           language,
			"sort_by":            "popularity.desc",
			"first_air_date.lte": time.Now().UTC().Format("2006-01-02"),
		}
	} else {
		p = napping.Params{
			"language":           language,
			"sort_by":            "popularity.desc",
			"first_air_date.lte": time.Now().UTC().Format("2006-01-02"),
			"with_genres":        genre,
		}
	}
	return ListShowsComplete("discover/tv", p)
}

func TopRatedShowsComplete(genre string, language string) Shows {
	return ListShowsComplete("tv/top_rated", napping.Params{"language": language})
}

func MostVotedShowsComplete(genre string, language string) Movies {
	return ListMoviesComplete("discover/tv", napping.Params{
		"language":           language,
		"sort_by":            "vote_count.desc",
		"first_air_date.lte": time.Now().UTC().Format("2006-01-02"),
		"with_genres":        genre,
	})
}

func GetTVGenres(language string) []*Genre {
	genres := GenreList{}
	rateLimiter.Call(func() {
        p := napping.Params{"api_key": apiKey, "language": language}.AsUrlValues()
		napping.Get(
			tmdbEndpoint+"genre/tv/list",
			&p,
			&genres,
			nil,
		)
	})
	return genres.Genres
}

func (show *Show) ToListItem() *xbmc.ListItem {
	year, _ := strconv.Atoi(strings.Split(show.ReleaseDate, "-")[0])

	item := &xbmc.ListItem{
		Label: show.OriginalName,
		Info: &xbmc.ListItemInfo{
			Year:          year,
			Count:         rand.Int(),
			Title:         show.OriginalName,
			OriginalTitle: show.Name,
			Plot:          show.Overview,
			PlotOutline:   show.Overview,
			Code:          show.ExternalIDs.IMDBId,
			Date:          show.ReleaseDate,
			Votes:         strconv.Itoa(show.VoteCount),
			Rating:        show.VoteAverage,
			TVShowTitle:   show.OriginalName,
			Premiered:     show.FirstAirDate,
		},
		Art: &xbmc.ListItemArt{
			FanArt: imageURL(show.BackdropPath, "w1280"),
			Poster: imageURL(show.PosterPath, "w500"),
		},
	}
	item.Thumbnail = item.Art.Poster
	item.Art.Thumbnail = item.Art.Poster

	if show.InProduction {
		item.Info.Status = "Continuing"
	} else {
		item.Info.Status = "Discontinued"
	}

	genres := make([]string, 0, len(show.Genres))
	for _, genre := range show.Genres {
		genres = append(genres, genre.Name)
	}
	item.Info.Genre = strings.Join(genres, " / ")

	for _, company := range show.ProductionCompanies {
		item.Info.Studio = company.Name
		break
	}
	if show.Credits != nil {
		item.Info.CastAndRole = make([][]string, 0)
		for _, cast := range show.Credits.Cast {
			item.Info.CastAndRole = append(item.Info.CastAndRole, []string{cast.Name, cast.Character})
		}
		directors := make([]string, 0)
		writers := make([]string, 0)
		for _, crew := range show.Credits.Crew {
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
