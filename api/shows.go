package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/providers/cli"
	"github.com/steeve/pulsar/trakt"
	"github.com/steeve/pulsar/xbmc"
)

func renderShows(shows trakt.ShowList, w http.ResponseWriter, r *http.Request) {
	items := make(xbmc.ListItems, 0, len(shows))
	for _, show := range shows {
		item := show.ToListItem()
		item.Path = UrlFor("show_seasons", "showId", show.TVDBId)
		items = append(items, item)
	}

	json.NewEncoder(w).Encode(xbmc.NewView("tvshows", items))
}

func PopularShows(w http.ResponseWriter, r *http.Request) {
	renderShows(trakt.TrendingShows(), w, r)
}

func SearchShows(w http.ResponseWriter, r *http.Request) {
	query := xbmc.Keyboard("", "Search Shows")
	if query != "" {
		renderShows(trakt.SearchShows(query), w, r)
	}
}

func ShowSeasons(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	show := trakt.NewShow(vars["showId"])
	seasons := show.Seasons()

	items := make(xbmc.ListItems, 0, len(seasons))
	for _, season := range seasons {
		if season.Season == 0 {
			continue
		}
		item := season.ToListItem()
		item.Path = UrlFor("show_season_episodes", "showId", show.TVDBId, "season", strconv.Itoa(season.Season))
		items = append(items, item)
	}

	json.NewEncoder(w).Encode(xbmc.NewView("seasons", items))
}

func ShowEpisodes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	show := trakt.NewShow(vars["showId"])
	seasonNumber, _ := strconv.Atoi(vars["season"])
	season := show.Season(seasonNumber)
	episodes := season.Episodes()

	items := make(xbmc.ListItems, 0, len(episodes))
	for _, episode := range episodes {
		item := episode.ToListItem()
		item.Path = UrlFor("show_episode_links",
			"showId", show.TVDBId,
			"season", strconv.Itoa(season.Season),
			"episode", strconv.Itoa(episode.Episode),
		)
		item.IsPlayable = true
		items = append(items, item)
	}

	json.NewEncoder(w).Encode(xbmc.NewView("episodes", items))
}

func ShowEpisodeLinks(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	log.Println("Searching links for TVDB Id:", vars["showId"])

	show := trakt.NewShow(vars["showId"])
	seasonNumber, _ := strconv.Atoi(vars["season"])
	episodeNumber, _ := strconv.Atoi(vars["episode"])

	episode := show.Season(seasonNumber).Episode(episodeNumber)

	log.Printf("Resolved %s to %s\n", vars["showId"], show.Title)

	searchers := []providers.EpisodeSearcher{
		cli.NewCLISearcher([]string{"python", "kat.py"}),
		cli.NewCLISearcher([]string{"python", "tpb.py"}),
		cli.NewCLISearcher([]string{"python", "torrentlookup.py"}),
	}

	torrents := providers.SearchEpisode(searchers, episode)

	choices := make([]string, 0, len(torrents))
	for _, torrent := range torrents {
		label := fmt.Sprintf("S:%d P:%d - %s",
			torrent.Seeds,
			torrent.Peers,
			torrent.Name,
		)
		choices = append(choices, label)
	}

	choice := xbmc.ListDialog("Choose your stream", choices...)
	if choice >= 0 {
		rUrl := fmt.Sprintf("plugin://plugin.video.xbmctorrent/play/%s", url.QueryEscape(torrents[choice].URI))
		http.Redirect(w, r, rUrl, http.StatusFound)
	}
}
