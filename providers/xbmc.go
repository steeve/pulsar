package providers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/tmdb"
	"github.com/steeve/pulsar/tvdb"
	"github.com/steeve/pulsar/util"
	"github.com/steeve/pulsar/xbmc"
)

const (
	// if >= 80% of episodes have absolute numbers, assume it's because we need it
	mixAbsoluteNumberPercentage = 0.8
)

type AddonSearcher struct {
	MovieSearcher
	EpisodeSearcher

	addonId string
	log     *logging.Logger
}

var cbLock = sync.RWMutex{}
var callbacks = map[string]chan []byte{}

func GetCallback() (string, chan []byte) {
	cid := strconv.Itoa(rand.Int())
	c := make(chan []byte, 1) // make sure we don't block clients when we write on it
	cbLock.Lock()
	callbacks[cid] = c
	cbLock.Unlock()
	return cid, c
}

func RemoveCallback(cid string) {
	cbLock.Lock()
	defer cbLock.Unlock()

	delete(callbacks, cid)
}

func CallbackHandler(ctx *gin.Context) {
	cid := ctx.Params.ByName("cid")
	cbLock.RLock()
	c, ok := callbacks[cid]
	cbLock.RUnlock()
	// maybe the callback was already removed because we were too slow,
	// it's fine.
	if !ok {
		return
	}
	RemoveCallback(cid)
	body, _ := ioutil.ReadAll(ctx.Request.Body)
	c <- body
	close(c)
}

func getSearchers() []interface{} {
	list := make([]interface{}, 0)
	for _, addon := range xbmc.GetAddons("xbmc.python.script", "executable", true).Addons {
		if strings.HasPrefix(addon.ID, "script.pulsar.") {
			list = append(list, NewAddonSearcher(addon.ID))
		}
	}
	return list
}

func GetMovieSearchers() []MovieSearcher {
	searchers := make([]MovieSearcher, 0)
	for _, searcher := range getSearchers() {
		searchers = append(searchers, searcher.(MovieSearcher))
	}
	return searchers
}

func GetEpisodeSearchers() []EpisodeSearcher {
	searchers := make([]EpisodeSearcher, 0)
	for _, searcher := range getSearchers() {
		searchers = append(searchers, searcher.(EpisodeSearcher))
	}
	return searchers
}

func GetSearchers() []Searcher {
	searchers := make([]Searcher, 0)
	for _, searcher := range getSearchers() {
		searchers = append(searchers, searcher.(Searcher))
	}
	return searchers
}

func NewAddonSearcher(addonId string) *AddonSearcher {
	return &AddonSearcher{
		addonId: addonId,
		log:     logging.MustGetLogger(fmt.Sprintf("AddonSearcher %s", addonId)),
	}
}

func (as *AddonSearcher) GetMovieSearchObject(movie *tmdb.Movie) *MovieSearchObject {
	year, _ := strconv.Atoi(strings.Split(movie.ReleaseDate, "-")[0])
	title := movie.OriginalTitle
	if title == "" {
		title = movie.Title
	}
	sObject := &MovieSearchObject{
		IMDBId: movie.IMDBId,
		Title:  NormalizeTitle(title),
		Year:   year,
		Titles: make(map[string]string),
	}
	for _, title := range movie.AlternativeTitles.Titles {
		sObject.Titles[strings.ToLower(title.ISO_3166_1)] = NormalizeTitle(title.Title)
	}
	return sObject
}

func (as *AddonSearcher) GetEpisodeSearchObject(show *tvdb.Show, episode *tvdb.Episode) *EpisodeSearchObject {
	absoluteNumber := 0
	if episode.AbsoluteNumber > 0 {
		totalEpisodes := 0
		totalEpisodesWithAbsoluteNumber := 0
		for _, season := range show.Seasons {
			totalEpisodes += len(season.Episodes)
			for _, episode := range season.Episodes {
				if episode.AbsoluteNumber > 0 {
					totalEpisodesWithAbsoluteNumber++
				}
			}
		}
		if float64(totalEpisodesWithAbsoluteNumber)/float64(totalEpisodes) >= mixAbsoluteNumberPercentage {
			absoluteNumber = episode.AbsoluteNumber
		}
	}

	seriesName := show.SeriesName
	tmdbFindResults := tmdb.Find(strconv.Itoa(show.Id), "tvdb_id")

	// FIXME: This can crash
	for _, result := range tmdbFindResults.TVResults {
		seriesName = result.Name
		break
	}

	return &EpisodeSearchObject{
		IMDBId:         show.ImdbId,
		TVDBId:         show.Id,
		Title:          NormalizeTitle(seriesName),
		Season:         episode.SeasonNumber,
		Episode:        episode.EpisodeNumber,
		AbsoluteNumber: absoluteNumber,
	}
}

func (as *AddonSearcher) call(method string, searchObject interface{}) []*bittorrent.Torrent {
	torrents := make([]*bittorrent.Torrent, 0)
	cid, c := GetCallback()
	cbUrl := fmt.Sprintf("%s/callbacks/%s", util.GetHTTPHost(), cid)

	payload := &SearchPayload{
		Method:       method,
		CallbackURL:  cbUrl,
		SearchObject: searchObject,
	}

	xbmc.ExecuteAddon(as.addonId, payload.String())

	select {
	case <-time.After(providerTimeout()):
		as.log.Info("Provider %s was too slow. Ignored.", as.addonId)
		RemoveCallback(cid)
	case result := <-c:
		json.Unmarshal(result, &torrents)
	}

	return torrents
}

func (as *AddonSearcher) SearchLinks(query string) []*bittorrent.Torrent {
	return as.call("search", query)
}

func (as *AddonSearcher) SearchMovieLinks(movie *tmdb.Movie) []*bittorrent.Torrent {
	return as.call("search_movie", as.GetMovieSearchObject(movie))
}

func (as *AddonSearcher) SearchEpisodeLinks(show *tvdb.Show, episode *tvdb.Episode) []*bittorrent.Torrent {
	return as.call("search_episode", as.GetEpisodeSearchObject(show, episode))
}
