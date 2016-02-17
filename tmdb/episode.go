package tmdb

import (
	"fmt"
	"time"
	"math/rand"

	"github.com/scakemyer/quasar/xbmc"
)

func (episodes EpisodeList) ToListItems(show *Show, season *Season) []*xbmc.ListItem {
	items := make([]*xbmc.ListItem, 0, len(episodes))
	if len(episodes) == 0 {
		return items
	}

	fanarts := make([]string, 0)
	for _, backdrop := range show.Images.Backdrops {
		fanarts = append(fanarts, ImageURL(backdrop.FilePath, "w1280"))
	}

	now := time.Now().UTC()
	for _, episode := range episodes {
		if episode.AirDate == "" {
			continue
		}
		firstAired, _ := time.Parse("2006-01-02", episode.AirDate)
		if firstAired.After(now) {
			continue
		}

		item := episode.ToListItem(show)

		if episode.StillPath != "" {
			item.Art.FanArt = ImageURL(episode.StillPath, "w1280")
			item.Art.Thumbnail = ImageURL(episode.StillPath, "w500")
		} else {
			if len(fanarts) > 0 {
				item.Art.FanArt = fanarts[rand.Intn(len(fanarts))]
			}
		}
		item.Art.Poster = ImageURL(season.Poster, "w500")

		items = append(items, item)
	}
	return items
}

func (episode *Episode) ToListItem(show *Show) *xbmc.ListItem {
	episodeLabel := fmt.Sprintf("%dx%02d %s", episode.SeasonNumber, episode.EpisodeNumber, episode.Name)

	item := &xbmc.ListItem{
		Label: episodeLabel,
		Info: &xbmc.ListItemInfo{
			Count:         rand.Int(),
			Title:         episodeLabel,
			OriginalTitle: episode.Name,
			Season:        episode.SeasonNumber,
			Episode:       episode.EpisodeNumber,
			TVShowTitle:   show.Title,
			Plot:          episode.Overview,
			PlotOutline:   episode.Overview,
			Rating:        episode.VoteAverage,
			Aired:         episode.AirDate,
		},
		Art: &xbmc.ListItemArt{},
	}

	return item
}
