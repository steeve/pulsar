package tmdb

import (
	"fmt"
	"path"
	"time"
	"math/rand"

	"github.com/jmcvetta/napping"
	"github.com/scakemyer/quasar/cache"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/xbmc"
)

func GetEpisode(showId int, seasonNumber int, episodeNumber int, language string) *Episode {
	var episode *Episode
	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := fmt.Sprintf("com.tmdb.episode.%d.%d.%s", showId, seasonNumber, episodeNumber, language)
	if err := cacheStore.Get(key, &episode); err != nil {
		rateLimiter.Call(func() {
			urlValues := napping.Params{
				"api_key": apiKey,
				"append_to_response": "credits,images,videos,external_ids",
				"language": language,
			}.AsUrlValues()
			resp, err := napping.Get(
				fmt.Sprintf("%stv/%d/season/%d/episode/%d", tmdbEndpoint, showId, seasonNumber, episodeNumber),
				&urlValues,
				&episode,
				nil,
			)
			if err != nil {
				log.Error(err.Error())
				xbmc.Notify("Quasar", "GetEpisode failed, check your logs.", config.AddonIcon())
			} else if resp.Status() != 200 {
				message := fmt.Sprintf("GetEpisode bad status: %d", resp.Status())
				log.Error(message)
				xbmc.Notify("Quasar", message, config.AddonIcon())
			}
		})

		if episode != nil {
			cacheStore.Set(key, episode, cacheTime)
		}
	}
	if episode == nil {
		return nil
	}
	return episode
}

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

	runtime := 1800
	if len(show.EpisodeRunTime) > 0 {
		runtime = show.EpisodeRunTime[len(show.EpisodeRunTime) - 1] * 60
	}

	item := &xbmc.ListItem{
		Label: episodeLabel,
		Info: &xbmc.ListItemInfo{
			Count:         rand.Int(),
			Title:         episodeLabel,
			OriginalTitle: episode.Name,
			Season:        episode.SeasonNumber,
			Episode:       episode.EpisodeNumber,
			TVShowTitle:   show.OriginalName,
			Plot:          episode.Overview,
			PlotOutline:   episode.Overview,
			Rating:        episode.VoteAverage,
			Aired:         episode.AirDate,
			Duration:      runtime,
		},
		Art: &xbmc.ListItemArt{},
	}

	return item
}
