package trakt

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/jmcvetta/napping"
	"github.com/steeve/pulsar/xbmc"
)

type ShowList []*Show

type Show struct {
	Title         string `json:"title"`
	Year          int    `json:"year"`
	URL           string `json:"url"`
	FirstAired    int    `json:"first_aired"`
	Country       string `json:"country"`
	Overview      string `json:"overview"`
	Runtime       int    `json:"runtime"`
	Status        string `json:"status"`
	Network       int    `json:"network"`
	AirDay        string `json:"air_day"`
	AirTime       string `json:"air_time"`
	Certification string `json:"certification"`
	IMDBId        string `json:"imdb_id"`

	TVDBId      string
	TVRageId    string
	TVDBIdRaw   interface{} `json:"tvdb_id"`
	TVRageIdRaw interface{} `json:"tvrage_id"`

	Images   *Images  `json:"images"`
	Watchers int      `json:"watchers"`
	Ratings  *Ratings `json:"ratings"`
	Genres   []string `json:"genres"`
}

type ShowSeason struct {
	Show          *Show   `json:"-"`
	Season        int     `json:"season"`
	TotalEpisodes int     `json:"episodes"`
	Images        *Images `json:"images"`
}

type ShowEpisode struct {
	Show          *Show       `json:"-"`
	Season        *ShowSeason `json:"-"`
	Episode       int         `json:"episode"`
	Number        int         `json:"number"`
	Title         string      `json:"title"`
	Overview      string      `json:"overview"`
	FirstAired    int64       `json:"first_aired"`
	FirstAiredUTC int64       `json:"first_aired_utc"`
	Screen        string      `json:"screen"`
	Images        *Images     `json:"images"`
	Ratings       *Ratings    `json:"ratings"`
}

func sanitizeIds(show *Show) {
	switch t := show.TVDBIdRaw.(type) {
	case string:
		show.TVDBId = t
	case float64:
		show.TVDBId = strconv.FormatUint(uint64(t), 10)
	}

	switch t := show.TVRageIdRaw.(type) {
	case string:
		show.TVRageId = t
	case float64:
		show.TVRageId = strconv.FormatUint(uint64(t), 10)
	}
}

func NewShow(TVDBId string) *Show {
	var show *Show
	napping.Get(fmt.Sprintf("%s/show/summary.json/%s/%s", ENDPOINT, APIKEY, TVDBId), nil, &show, nil)
	sanitizeIds(show)
	return show
}

func SearchShows(query string) ShowList {
	var shows ShowList
	napping.Get(fmt.Sprintf("%s/search/shows.json/%s", ENDPOINT, APIKEY),
		&napping.Params{"query": query, "limit": SearchLimit},
		&shows,
		nil)
	for _, show := range shows {
		sanitizeIds(show)
	}
	return shows
}

func TrendingShows() ShowList {
	var shows ShowList
	napping.Get(fmt.Sprintf("%s/shows/trending.json/%s", ENDPOINT, APIKEY), nil, &shows, nil)
	for _, show := range shows {
		sanitizeIds(show)
	}
	return shows
}

func (show *Show) Seasons() []*ShowSeason {
	var seasons []*ShowSeason
	napping.Get(fmt.Sprintf("%s/show/seasons.json/%s/%s", ENDPOINT, APIKEY, show.TVDBId), nil, &seasons, nil)
	for _, season := range seasons {
		season.Show = show
	}
	return seasons
}

func (show *Show) Season(season int) *ShowSeason {
	return &ShowSeason{
		Show:   show,
		Season: season,
	}
}

func (season *ShowSeason) Episodes() []*ShowEpisode {
	var episodes []*ShowEpisode
	napping.Get(fmt.Sprintf("%s/show/season.json/%s/%s/%d", ENDPOINT, APIKEY, season.Show.TVDBId, season.Season), nil, &episodes, nil)

	var airedEpisodes []*ShowEpisode
	now := time.Now().UTC().Unix()
	for _, episode := range episodes {
		if episode.FirstAiredUTC <= now {
			episode.Show = season.Show
			episode.Season = season
			airedEpisodes = append(airedEpisodes, episode)
		}
	}

	return airedEpisodes
}

func (season *ShowSeason) Episode(episode int) *ShowEpisode {
	return &ShowEpisode{
		Show:    season.Show,
		Season:  season,
		Episode: episode,
	}
}

func (show *Show) ToListItem() *xbmc.ListItem {
	return &xbmc.ListItem{
		Label: show.Title,
		Info: &xbmc.ListItemInfo{
			Count:       rand.Int(),
			Title:       show.Title,
			Genre:       strings.Join(show.Genres, " / "),
			Plot:        show.Overview,
			PlotOutline: show.Overview,
			Rating:      float32(show.Ratings.Percentage) / 10,
			Duration:    show.Runtime,
			Code:        show.TVDBId,
		},
		Art: &xbmc.ListItemArt{
			Poster: show.Images.Poster,
			FanArt: show.Images.FanArt,
			Banner: show.Images.Banner,
		},
	}
}

func (season *ShowSeason) ToListItem() *xbmc.ListItem {
	seasonLabel := fmt.Sprintf("Season %d", season.Season)
	return &xbmc.ListItem{
		Label: seasonLabel,
		Info: &xbmc.ListItemInfo{
			Count:  rand.Int(),
			Title:  seasonLabel,
			Season: season.Season,
		},
		Art: &xbmc.ListItemArt{
			Poster: season.Images.Poster,
			FanArt: season.Show.Images.FanArt,
			Banner: season.Show.Images.Banner,
		},
	}
}

func (episode *ShowEpisode) ToListItem() *xbmc.ListItem {
	title := fmt.Sprintf("%dx%02d %s", episode.Season.Season, episode.Episode, episode.Title)
	return &xbmc.ListItem{
		Label:     title,
		Thumbnail: episode.Images.Screen,
		Info: &xbmc.ListItemInfo{
			Count:       rand.Int(),
			Title:       title,
			Plot:        episode.Overview,
			PlotOutline: episode.Overview,
			Rating:      float32(episode.Ratings.Percentage) / 10,
			Episode:     episode.Episode,
			Season:      episode.Season.Season,
		},
		Art: &xbmc.ListItemArt{
			Thumbnail: episode.Images.Screen,
			FanArt:    episode.Season.Show.Images.FanArt,
			Banner:    episode.Season.Show.Images.Banner,
		},
	}
}
