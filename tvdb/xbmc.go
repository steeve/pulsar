package tvdb

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/steeve/pulsar/xbmc"
)

func imageURL(path string) string {
	return tvdbUrl + "/banners/" + path
}

func (seasons SeasonList) ToListItems() []*xbmc.ListItem {
	items := make([]*xbmc.ListItem, 0, len(seasons))

	fanarts := make([]string, 0)
	for _, banner := range seasons[0].Show.Banners {
		if banner.BannerType == "fanart" &&
			banner.BannerType2 == "1280x720" {
			fanarts = append(fanarts, imageURL(banner.BannerPath))
		}
	}
	now := time.Now().UTC()
	for _, season := range seasons {
		if len(season.Episodes) == 0 {
			continue
		}
		airedDateTime := fmt.Sprintf("%s %s EST", season.Episodes[0].FirstAired, season.Show.AirsTime)
		firstAired, _ := time.Parse("2006-01-02 3:04 PM MST", airedDateTime)
		if firstAired.Add(time.Duration(season.Show.Runtime) * time.Minute).After(now) {
			continue
		}
		item := season.ToListItem()
		item.Art.FanArt = fanarts[rand.Int()%len(fanarts)]
		items = append(items, item)
	}
	return items
}

func (episodes EpisodeList) ToListItems() []*xbmc.ListItem {
	items := make([]*xbmc.ListItem, 0, len(episodes))
	if len(episodes) == 0 {
		return items
	}

	fanarts := make([]string, 0)
	for _, banner := range episodes[0].Show.Banners {
		if banner.BannerType == "fanart" &&
			banner.BannerType2 == "1280x720" {
			fanarts = append(fanarts, imageURL(banner.BannerPath))
		}
	}
	now := time.Now().UTC()
	for _, episode := range episodes {
		airedDateTime := fmt.Sprintf("%s %s EST", episode.FirstAired, episode.Show.AirsTime)
		firstAired, _ := time.Parse("2006-01-02 3:04 PM MST", airedDateTime)
		if firstAired.Add(time.Duration(episode.Show.Runtime) * time.Minute).After(now) {
			continue
		}
		item := episode.ToListItem()
		item.Art.FanArt = fanarts[rand.Int()%len(fanarts)]
		items = append(items, item)
	}
	return items
}

func (season *Season) ToListItem() *xbmc.ListItem {
	show := season.Show

	name := fmt.Sprintf("Season %d", season.Season)
	if season.Season == 0 {
		name = "Specials"
	}

	item := &xbmc.ListItem{
		Label: name,
		Info: &xbmc.ListItemInfo{
			Count:         rand.Int(),
			Title:         name,
			OriginalTitle: name,
			Season:        season.Season,
			TVShowTitle:   show.SeriesName,
		},
		Art: &xbmc.ListItemArt{},
	}

	for _, banner := range season.Show.Banners {
		if banner.BannerType2 == "season" &&
			banner.Season == season.Season &&
			banner.Language == season.Show.Language &&
			item.Art.Poster == "" {
			item.Art.Poster = imageURL(banner.BannerPath)
			item.Art.Thumbnail = item.Art.Poster
			item.Thumbnail = item.Art.Poster
			break
		}
	}

	item.Info.Genre = strings.Replace(strings.Trim(show.Genre, "|"), "|", " / ", -1)

	return item
}

func (episode *Episode) ToListItem() *xbmc.ListItem {
	show := episode.Show

	episodeLabel := fmt.Sprintf("%dx%02d %s", episode.SeasonNumber, episode.EpisodeNumber, episode.EpisodeName)

	item := &xbmc.ListItem{
		Label: episodeLabel,
		Info: &xbmc.ListItemInfo{
			Count:         rand.Int(),
			Title:         episodeLabel,
			OriginalTitle: episode.EpisodeName,
			Season:        episode.SeasonNumber,
			Episode:       episode.EpisodeNumber,
			TVShowTitle:   show.SeriesName,
			Plot:          episode.Overview,
			PlotOutline:   episode.Overview,
		},
		Art: &xbmc.ListItemArt{
			Poster: imageURL(episode.FileName),
		},
	}

	return item
}
