package tmdb

import (
	"fmt"
	"path"
	"sync"
	"time"
	"strconv"
	"strings"
	"math/rand"
	"encoding/json"
	"runtime"

	"github.com/jmcvetta/napping"
	"github.com/scakemyer/quasar/cache"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/xbmc"
)

func LogError(err error) {
	if err != nil {
		pc, fn, line, _ := runtime.Caller(1)
		log.Errorf("in %s[%s:%d] %#v: %v)", runtime.FuncForPC(pc).Name(), fn, line, err, err)
	}
}

func GetShow(showId int, language string) *Show {
	var show *Show
	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := fmt.Sprintf("com.tmdb.show.%d.%s", showId, language)
	if err := cacheStore.Get(key, &show); err != nil {
		rateLimiter.Call(func() {
			urlValues := napping.Params{
				"api_key": apiKey,
				"append_to_response": "credits,images,alternative_titles,translations,external_ids",
				"language": language,
			}.AsUrlValues()
			resp, err := napping.Get(
				tmdbEndpoint + "tv/" + strconv.Itoa(showId),
				&urlValues,
				&show,
				nil,
			)
			if err != nil {
				switch e := err.(type) {
					case *json.UnmarshalTypeError:
						log.Errorf("UnmarshalTypeError: Value[%s] Type[%v] Offset[%d] for %d", e.Value, e.Type, e.Offset, showId)
					case *json.InvalidUnmarshalError:
						log.Errorf("InvalidUnmarshalError: Type[%v]", e.Type)
					default:
						log.Error(err.Error())
				}
				LogError(err)
				xbmc.Notify("Quasar", "GetShow failed, check your logs.", config.AddonIcon())
			} else if resp.Status() != 200 {
				message := fmt.Sprintf("GetShow bad status: %d", resp.Status())
				log.Error(message)
				xbmc.Notify("Quasar", message, config.AddonIcon())
			}
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

func SearchShows(query string, language string, page int) Shows {
	var results EntityList
	rateLimiter.Call(func() {
		urlValues := napping.Params{
			"api_key": apiKey,
			"query": query,
			"page": strconv.Itoa(startPage + page),
		}.AsUrlValues()
		resp, err := napping.Get(
			tmdbEndpoint + "search/tv",
			&urlValues,
			&results,
			nil,
		)
		if err != nil {
			log.Error(err.Error())
			xbmc.Notify("Quasar", "SearchShows failed, check your logs.", config.AddonIcon())
		} else if resp.Status() != 200 {
			message := fmt.Sprintf("SearchShows bad status: %d", resp.Status())
			log.Error(message)
			xbmc.Notify("Quasar", message, config.AddonIcon())
		}
	})
	tmdbIds := make([]int, 0, len(results.Results))
	for _, entity := range results.Results {
		tmdbIds = append(tmdbIds, entity.Id)
	}
	return GetShows(tmdbIds, language)
}

func ListShows(endpoint string, params napping.Params, page int) (shows Shows) {
	var results *EntityList

	params["page"] = strconv.Itoa(startPage + page)
	params["api_key"] = apiKey
	p := params.AsUrlValues()

	rateLimiter.Call(func() {
		resp, err := napping.Get(
			tmdbEndpoint + endpoint,
			&p,
			&results,
			nil,
		)
		if err != nil {
			log.Error(err.Error())
			xbmc.Notify("Quasar", "ListShows failed, check your logs.", config.AddonIcon())
		} else if resp.Status() != 200 {
			message := fmt.Sprintf("ListShows bad status: %d", resp.Status())
			log.Error(message)
			xbmc.Notify("Quasar", message, config.AddonIcon())
		}
	})
	if results != nil {
		for _, show := range results.Results {
			shows = append(shows, GetShow(show.Id, params["language"]))
		}
	}
	return shows
}

func PopularShows(genre string, language string, page int) Shows {
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
	return ListShows("discover/tv", p, page)
}

func RecentShows(genre string, language string, page int) Shows {
	var p napping.Params
	if genre == "" {
		p = napping.Params{
			"language":           language,
			"sort_by":            "first_air_date.desc",
			"first_air_date.lte": time.Now().UTC().Format("2006-01-02"),
		}
	} else {
		p = napping.Params{
			"language":           language,
			"sort_by":            "first_air_date.desc",
			"first_air_date.lte": time.Now().UTC().Format("2006-01-02"),
			"with_genres":        genre,
		}
	}
	return ListShows("discover/tv", p, page)
}

func RecentEpisodes(genre string, language string, page int) Shows {
	var p napping.Params

	if genre == "" {
		p = napping.Params{
			"language":           language,
			"air_date.gte": time.Now().UTC().AddDate(0, 0, -3).Format("2006-01-02"),
			"first_air_date.lte": time.Now().UTC().Format("2006-01-02"),
		}
	} else {
		p = napping.Params{
			"language":           language,
			"air_date.gte": time.Now().UTC().AddDate(0, 0, -3).Format("2006-01-02"),
			"first_air_date.lte": time.Now().UTC().Format("2006-01-02"),
			"with_genres":        genre,
		}
	}
	return ListShows("discover/tv", p, page)
}

func TopRatedShows(genre string, language string, page int) Shows {
	return ListShows("tv/top_rated", napping.Params{"language": language}, page)
}

func MostVotedShows(genre string, language string, page int) Shows {
	return ListShows("discover/tv", napping.Params{
		"language":           language,
		"sort_by":            "vote_count.desc",
		"first_air_date.lte": time.Now().UTC().Format("2006-01-02"),
		"with_genres":        genre,
	}, page)
}

func GetTVGenres(language string) []*Genre {
	genres := GenreList{}
	rateLimiter.Call(func() {
		urlValues := napping.Params{
			"api_key": apiKey,
			"language": language,
		}.AsUrlValues()
		resp, err := napping.Get(
			tmdbEndpoint + "genre/tv/list",
			&urlValues,
			&genres,
			nil,
		)
		if err != nil {
			log.Error(err.Error())
			xbmc.Notify("Quasar", "GetTVGenres failed, check your logs.", config.AddonIcon())
		} else if resp.Status() != 200 {
			message := fmt.Sprintf("GetTVGenres bad status: %d", resp.Status())
			log.Error(message)
			xbmc.Notify("Quasar", message, config.AddonIcon())
		}
	})
	return genres.Genres
}

func (show *Show) ToListItem() *xbmc.ListItem {
	year, _ := strconv.Atoi(strings.Split(show.FirstAirDate, "-")[0])

	name := show.Name
	if config.Get().UseOriginalTitle && show.OriginalName != "" {
		name = show.OriginalName
	}

	item := &xbmc.ListItem{
		Label: name,
		Info: &xbmc.ListItemInfo{
			Year:          year,
			Count:         rand.Int(),
			Title:         name,
			OriginalTitle: show.OriginalName,
			Plot:          show.Overview,
			PlotOutline:   show.Overview,
			Code:          show.ExternalIDs.IMDBId,
			Date:          show.FirstAirDate,
			Votes:         strconv.Itoa(show.VoteCount),
			Rating:        show.VoteAverage,
			TVShowTitle:   show.OriginalName,
			Premiered:     show.FirstAirDate,
		},
		Art: &xbmc.ListItemArt{
			FanArt: ImageURL(show.BackdropPath, "w1280"),
			Poster: ImageURL(show.PosterPath, "w500"),
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
