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
	"github.com/steeve/pulsar/trakt"
	"github.com/steeve/pulsar/util"
	"github.com/steeve/pulsar/xbmc"
)

type AddonSearcher struct {
	MovieSearcher
	EpisodeSearcher

	addonId string
	log     *logging.Logger
}

const (
	DefaultTimeout = 4 * time.Second
)

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

func NewAddonSearcher(addonId string) *AddonSearcher {
	return &AddonSearcher{
		addonId: addonId,
		log:     logging.MustGetLogger(fmt.Sprintf("AddonSearcher %s", addonId)),
	}
}

func (as *AddonSearcher) call(method string, searchObject interface{}) []*bittorrent.Torrent {
	torrents := []*bittorrent.Torrent{}
	cid, c := GetCallback()
	cbUrl := fmt.Sprintf("%s/callbacks/%s", util.GetHTTPHost(), cid)

	payload := &SearchPayload{
		Method:       method,
		CallbackURL:  cbUrl,
		SearchObject: searchObject,
	}

	xbmc.ExecuteAddon(as.addonId, payload.String())

	select {
	case <-time.After(DefaultTimeout):
		as.log.Info("Provider %s was too slow. Ignored.", as.addonId)
		RemoveCallback(cid)
	case result := <-c:
		json.Unmarshal(result, &torrents)
	}

	return torrents
}

func (as *AddonSearcher) SearchLinks(query string) []*bittorrent.Torrent {
	return as.call("search", QuerySearchObject{
		Query: query,
	})
}

func (as *AddonSearcher) SearchMovieLinks(movie *tmdb.Movie) []*bittorrent.Torrent {
	year, _ := strconv.Atoi(strings.Split(movie.ReleaseDate, "-")[0])
	title := movie.OriginalTitle
	if title == "" {
		title = movie.Title
	}
	sObject := MovieSearchObject{
		IMDBId: movie.IMDBId,
		Title:  NormalizeTitle(title),
		Year:   year,
		Titles: make(map[string]string),
	}
	for _, title := range movie.AlternativeTitles.Titles {
		sObject.Titles[strings.ToLower(title.ISO_3166_1)] = NormalizeTitle(title.Title)
	}
	return as.call("search_movie", sObject)
}

func (as *AddonSearcher) SearchEpisodeLinks(episode *trakt.ShowEpisode) []*bittorrent.Torrent {
	sObject := EpisodeSearchObject{
		IMDBId:  episode.Show.IMDBId,
		TVDBId:  episode.Show.TVDBId,
		Title:   NormalizeTitle(episode.Show.Title),
		Season:  episode.Season.Season,
		Episode: episode.Episode,
	}
	return as.call("search_episode", sObject)
}
