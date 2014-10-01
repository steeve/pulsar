package providers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"regexp"
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

func (as *AddonSearcher) call(method string, args ...interface{}) []*bittorrent.Torrent {
	torrents := []*bittorrent.Torrent{}
	cid, c := GetCallback()
	cbUrl := fmt.Sprintf("%s/callbacks/%s", util.GetHTTPHost(), cid)

	payload := &SearchPayload{
		Method:      method,
		CallbackURL: cbUrl,
		Args:        args,
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
	return as.call("search", query)
}

func (as *AddonSearcher) SearchMovieLinks(movie *tmdb.Movie) []*bittorrent.Torrent {
	year := strings.Split(movie.ReleaseDate, "-")[0]
	title := movie.OriginalTitle
	if title == "" {
		title = movie.Title
	}
	return as.call("search_movie", movie.IMDBId, title, year)
}

func (as *AddonSearcher) SearchEpisodeLinks(episode *trakt.ShowEpisode) []*bittorrent.Torrent {
	normalizedTitle := episode.Show.Title
	normalizedTitle = strings.ToLower(normalizedTitle)
	normalizedTitle = regexp.MustCompile(`'`).ReplaceAllString(normalizedTitle, "")
	normalizedTitle = regexp.MustCompile(`\(\d+\)`).ReplaceAllString(normalizedTitle, " ")
	normalizedTitle = regexp.MustCompile(`(\W+|\s+)`).ReplaceAllString(normalizedTitle, " ")
	normalizedTitle = strings.TrimSpace(normalizedTitle)

	return as.call("search_episode",
		episode.Show.IMDBId,
		episode.Show.TVDBId,
		normalizedTitle,
		episode.Season.Season,
		episode.Episode)
}
