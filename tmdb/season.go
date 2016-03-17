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

func GetSeason(showId int, seasonNumber int, language string) *Season {
	var season *Season
	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := fmt.Sprintf("com.tmdb.season.%d.%d.%s", showId, seasonNumber, language)
	if err := cacheStore.Get(key, &season); err != nil {
		rateLimiter.Call(func() {
			urlValues := napping.Params{
				"api_key": apiKey,
				"append_to_response": "credits,images,videos,external_ids",
				"language": language,
			}.AsUrlValues()
			resp, err := napping.Get(
				fmt.Sprintf("%stv/%d/season/%d", tmdbEndpoint, showId, seasonNumber),
				&urlValues,
				&season,
				nil,
			)
			if err != nil {
				log.Error(err.Error())
				xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
			} else if resp.Status() != 200 {
				message := fmt.Sprintf("GetSeason bad status: %d", resp.Status())
				log.Error(message)
				xbmc.Notify("Quasar", message, config.AddonIcon())
			}
		})
		season.EpisodeCount = len(season.Episodes)

		// Fix for shows that have translations but return empty strings
		// for episode names and overviews.
		// We detect if episodes have their name filled, and if not re-query
		// with no language set.
		// See https://github.com/scakemyer/plugin.video.quasar/issues/249
		if season.EpisodeCount > 0 {
			for index := 0; index < season.EpisodeCount; index++ {
				if season.Episodes[index].Name == "" {
					season.Episodes[index] = GetEpisode(showId, seasonNumber, index + 1, "")
				}
			}
		}

		if season != nil {
			cacheStore.Set(key, season, cacheTime)
		}
	}
	if season == nil {
		return nil
	}
	return season
}

func (seasons SeasonList) ToListItems(show *Show) []*xbmc.ListItem {
	items := make([]*xbmc.ListItem, 0, len(seasons))

	fanarts := make([]string, 0)
	for _, backdrop := range show.Images.Backdrops {
		fanarts = append(fanarts, ImageURL(backdrop.FilePath, "w1280"))
	}

	now := time.Now().UTC()
	for _, season := range seasons {
		if season.EpisodeCount == 0 {
			continue
		}
		firstAired, _ := time.Parse("2006-01-02", season.AirDate)
		if firstAired.After(now) {
			continue
		}

		item := season.ToListItem(show)

		if len(fanarts) > 0 {
			item.Art.FanArt = fanarts[rand.Intn(len(fanarts))]
		}

		items = append(items, item)
	}
	return items
}

func (season *Season) ToListItem(show *Show) *xbmc.ListItem {
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
			TVShowTitle:   show.OriginalName,
		},
		Art: &xbmc.ListItemArt{},
	}

	if season.Poster != "" {
		item.Art.Poster = ImageURL(season.Poster, "w500")
		item.Art.Thumbnail = ImageURL(season.Poster, "w500")
	}

	fanarts := make([]string, 0)
	for _, backdrop := range show.Images.Backdrops {
		fanarts = append(fanarts, ImageURL(backdrop.FilePath, "w1280"))
	}
	if len(fanarts) > 0 {
		item.Art.FanArt = fanarts[rand.Intn(len(fanarts))]
	}

	// FIXME
	item.Info.Genre = show.Genres[0].Name

	return item
}

func (s SeasonList) Len() int           { return len(s) }
func (s SeasonList) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s SeasonList) Less(i, j int) bool { return s[i].Season < s[j].Season }
