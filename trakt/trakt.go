package trakt

import (
	"fmt"
	"math"
	"time"
	"bytes"
	"errors"
	"strconv"
	"net/url"
	"net/http"

	"github.com/op/go-logging"
	"github.com/jmcvetta/napping"
	"github.com/scakemyer/quasar/cloudhole"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/xbmc"
)

const (
	ApiUrl       = "https://api-v2launch.trakt.tv"
	ClientId     = "4407ab20a3a971e7c92d4996b36b76d0312ea085cb139d7c38a1a4c9f8428f60"
	ClientSecret = "83f5993015942fe1320772c9c9886dce08252fa95445afab81a1603f8671e490"
	ApiVersion   = "2"
)

var log = logging.MustGetLogger("trakt")

var (
	clearance, _ = cloudhole.GetClearance()
	retries      = 0
	scrobbleTime = float64(0)
	scrobbleEnd  = float64(0)
)

type Object struct {
	Title string `json:"title"`
	Year  int    `json:"year"`
	IDs   *IDs   `json:"ids"`
}

type Movie struct {
	Object

	Released      string      `json:"released"`
	URL           string      `json:"homepage"`
	Trailer       string      `json:"trailer"`
	Runtime       int         `json:"runtime"`
	TagLine       string      `json:"tagline"`
	Overview      string      `json:"overview"`
	Certification string      `json:"certification"`
	Rating        float32     `json:"rating"`
	Votes         int         `json:"votes"`
	Genres        []string    `json:"genres"`
	Language      string      `json:"language"`
	Translations  []string    `json:"available_translations"`

	Images        *Images     `json:"images"`
}

type Show struct {
	Object

	FirstAired    int         `json:"first_aired"`
	URL           string      `json:"homepage"`
	Trailer       string      `json:"trailer"`
	Runtime       int         `json:"runtime"`
	Overview      string      `json:"overview"`
	Certification string      `json:"certification"`
	Status        string      `json:"status"`
	Network       int         `json:"network"`
	AiredEpisodes int         `json:"aired_episodes"`
	Airs          *Airs       `json:"airs"`
	Rating        float32     `json:"rating"`
	Votes         int         `json:"votes"`
	Genres        []string    `json:"genres"`
	Country       string      `json:"country"`
	Language      string      `json:"language"`
	Translations  []string    `json:"available_translations"`

	Images        *Images `json:"images"`
}

type Season struct {
	// Show          *Show   `json:"-"`
	Number        int         `json:"number"`
	Overview      string      `json:"overview"`
	EpisodeCount  int         `json:"episode_count"`
	AiredEpisodes int         `json:"aired_episodes"`
	Rating        float32     `json:"rating"`
	Votes         int         `json:"votes"`

	Images        *Images     `json:"images"`
	IDs           *IDs        `json:"ids"`
}

type Episode struct {
	// Show          *Show       `json:"-"`
	// Season        *ShowSeason `json:"-"`
	Number        int         `json:"number"`
	Season        int         `json:"season"`
	Title         string      `json:"title"`
	Overview      string      `json:"overview"`
	Absolute      int         `json:"number_abs"`
	FirstAired    string      `json:"first_aired"`
	Translations  []string    `json:"available_translations"`

	Rating        float32     `json:"rating"`
	Votes         int         `json:"votes"`

	Images        *Images     `json:"images"`
	IDs           *IDs        `json:"ids"`
}

type Airs struct {
	Day           string      `json:"day"`
	Time          string      `json:"time"`
	Timezone      string      `json:"timezone"`
}

type Movies struct {
	Watchers int    `json:"watchers"`
	Movie    *Movie `json:"movie"`
}

type Shows struct {
	Watchers int   `json:"watchers"`
	Show     *Show `json:"show"`
}

type Watchlist struct {
	Movies   []*Movie   `json:"movies"`
	Shows    []*Show    `json:"shows"`
	Episodes []*Episode `json:"episodes"`
}

type WatchlistMovie struct {
	ListedAt string  `json:"listed_at"`
	Type     string  `json:"type"`
	Movie    *Movie  `json:"movie"`
}

type WatchlistShow struct {
	ListedAt string  `json:"listed_at"`
	Type     string  `json:"type"`
	Show     *Show   `json:"show"`
}

type WatchlistSeason struct {
	ListedAt string  `json:"listed_at"`
	Type     string  `json:"type"`
	Season   *Object `json:"season"`
	Show     *Object `json:"show"`
}

type WatchlistEpisode struct {
	ListedAt string   `json:"listed_at"`
	Type     string   `json:"type"`
	Episode  *Episode `json:"episode"`
	Show     *Object  `json:"show"`
}

type CollectionMovie struct {
	CollectedAt string `json:"collected_at"`
	Movie       *Movie `json:"movie"`
}

type CollectionShow struct {
	CollectedAt string             `json:"last_collected_at"`
	Show        *Show              `json:"show"`
	Seasons     []*CollectedSeason `json:"seasons"`
}

type CollectedSeason struct {
	Number   int                 `json:"number"`
	Episodes []*CollectedEpisode `json:"episodes"`
}

type CollectedEpisode struct {
	CollectedAt string `json:"collected_at"`
	Number      int    `json:"number"`
}

type Images struct {
	Poster     *Sizes `json:"poster"`
	FanArt     *Sizes `json:"fanart"`
	ScreenShot *Sizes `json:"screenshot"`
	HeadShot   *Sizes `json:"headshot"`
	Logo       *Sizes `json:"logo"`
	ClearArt   *Sizes `json:"clearart"`
	Banner     *Sizes `json:"banner"`
	Thumbnail  *Sizes `json:"thumb"`
	Avatar     *Sizes `json:"avatar"`
}

type Sizes struct {
	Full      string `json:"full"`
	Medium    string `json:"medium"`
	Thumbnail string `json:"thumb"`
}

type IDs struct {
  Trakt  int    `json:"trakt"`
  IMDB   string `json:"imdb"`
	TMDB   int    `json:"tmdb"`
  TVDB   int    `json:"tvdb"`
	TVRage int    `json:"tvrage"`
  Slug   string `json:"slug"`
}

type Code struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type Token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func newClearance() (err error) {
	log.Warningf("CloudFlared! User-Agent: %s - Cookies: %s", clearance.UserAgent, clearance.Cookies)

	if config.Get().CloudHoleKey == "" {
		retries = 3
		return errors.New("CloudFlared! Set your CloudHole API key.")
	}

	clearance, err = cloudhole.GetClearance()
	if err == nil {
		log.Noticef("New clearance: %s - %s", clearance.UserAgent, clearance.Cookies)
	} else {
		retries = 3
	}

	return err
}

func Get(endPoint string, params url.Values) (resp *napping.Response, err error) {
	header := http.Header{
		"Content-type": []string{"application/json"},
		"trakt-api-key": []string{ClientId},
		"trakt-api-version": []string{ApiVersion},
		"User-Agent": []string{clearance.UserAgent},
		"Cookie": []string{clearance.Cookies},
	}

	req := napping.Request{
		Url: fmt.Sprintf("%s/%s", ApiUrl, endPoint),
		Method: "GET",
		Params: &params,
		Header: &header,
	}

	resp, err = napping.Send(&req)
	if err == nil && resp.Status() == 403 && retries < 3 {
		retries += 1
		err = newClearance()
		if err == nil {
			resp, err = Get(endPoint, params)
		}
	}

	return resp, err
}

func GetWithAuth(endPoint string, params url.Values) (resp *napping.Response, err error) {
	header := http.Header{
		"Content-type": []string{"application/json"},
		"Authorization": []string{fmt.Sprintf("Bearer %s", config.Get().TraktToken)},
		"trakt-api-key": []string{ClientId},
		"trakt-api-version": []string{ApiVersion},
		"User-Agent": []string{clearance.UserAgent},
		"Cookie": []string{clearance.Cookies},
	}

	req := napping.Request{
		Url: fmt.Sprintf("%s/%s", ApiUrl, endPoint),
		Method: "GET",
		Params: &params,
		Header: &header,
	}

	resp, err = napping.Send(&req)
	if err == nil && resp.Status() == 403 && retries < 3 {
		retries += 1
		err = newClearance()
		if err == nil {
			resp, err = GetWithAuth(endPoint, params)
		}
	}

	return resp, err
}

func Post(endPoint string, payload *bytes.Buffer) (resp *napping.Response, err error) {
	header := http.Header{
		"Content-type": []string{"application/json"},
		"Authorization": []string{fmt.Sprintf("Bearer %s", config.Get().TraktToken)},
		"trakt-api-key": []string{ClientId},
		"trakt-api-version": []string{ApiVersion},
		"User-Agent": []string{clearance.UserAgent},
		"Cookie": []string{clearance.Cookies},
	}

	req := napping.Request{
		Url: fmt.Sprintf("%s/%s", ApiUrl, endPoint),
		Method: "POST",
		RawPayload: true,
		Payload: payload,
		Header: &header,
	}

	resp, err = napping.Send(&req)
	if err == nil && resp.Status() == 403 && retries < 3 {
		retries += 1
		err = newClearance()
		if err == nil {
			resp, err = Post(endPoint, payload)
		}
	}

	return resp, err
}

func GetCode() (code *Code, err error) {
	endPoint := "oauth/device/code"
	header := http.Header{
		"Content-type": []string{"application/json"},
		"User-Agent": []string{clearance.UserAgent},
		"Cookie": []string{clearance.Cookies},
	}
	params := napping.Params{
		"client_id": ClientId,
	}.AsUrlValues()

	req := napping.Request{
		Url: fmt.Sprintf("%s/%s", ApiUrl, endPoint),
		Method: "POST",
		Params: &params,
		Header: &header,
	}

	resp, err := napping.Send(&req)
	if err == nil && resp.Status() == 403 && retries < 3 {
		retries += 1
		err = newClearance()
		if err == nil {
			code, err = GetCode()
		}
	} else {
		resp.Unmarshal(&code)
	}

	if err == nil && resp.Status() != 200 {
		err = errors.New(fmt.Sprintf("Unable to get Trakt code: %d", resp.Status()))
	}

	return code, err
}

func GetToken(code string) (resp *napping.Response, err error) {
	endPoint := "oauth/device/token"
	header := http.Header{
		"Content-type": []string{"application/json"},
		"User-Agent": []string{clearance.UserAgent},
		"Cookie": []string{clearance.Cookies},
	}
	params := napping.Params{
		"code": code,
		"client_id": ClientId,
		"client_secret": ClientSecret,
	}.AsUrlValues()

	req := napping.Request{
		Url: fmt.Sprintf("%s/%s", ApiUrl, endPoint),
		Method: "POST",
		Params: &params,
		Header: &header,
	}

	resp, err = napping.Send(&req)
	if err == nil && resp.Status() == 403 && retries < 3 {
		retries += 1
		err = newClearance()
		if err == nil {
			resp, err = GetToken(code)
		}
	}

	return resp, err
}

func PollToken(code *Code) (token *Token, err error) {
	startInterval := code.Interval
	interval := time.NewTicker(time.Duration(startInterval) * time.Second)
	defer interval.Stop()
	expired := time.NewTicker(time.Duration(code.ExpiresIn) * time.Second)
	defer expired.Stop()

	for {
		select {
		case <-interval.C:
			resp, err := GetToken(code.DeviceCode)
			if err != nil {
				return nil, err
			}
			if resp.Status() == 200 {
				resp.Unmarshal(&token)
				return token, err
			} else if resp.Status() == 400 {
				break
			} else if resp.Status() == 404 {
				err = errors.New("Invalid device code.")
				return nil, err
			} else if resp.Status() == 409 {
				err = errors.New("Code already used.")
				return nil, err
			} else if resp.Status() == 410 {
				err = errors.New("Code expired.")
				return nil, err
			} else if resp.Status() == 418 {
				err = errors.New("Code denied.")
				return nil, err
			} else if resp.Status() == 429 {
				// err = errors.New("Polling too quickly.")
				interval.Stop()
				interval = time.NewTicker(time.Duration(startInterval + 5) * time.Second)
				break
			}

		case <-expired.C:
			err = errors.New("Code expired, please try again.")
			return nil, err
		}
	}
}

func Authorize(fromSettings bool) error {
	code, err := GetCode()

	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
		return err
	}
	log.Noticef("Got code for %s: %s", code.VerificationURL, code.UserCode)

	if xbmc.Dialog("LOCALIZE[30058]", fmt.Sprintf("Visit %s and enter your code: %s", code.VerificationURL, code.UserCode)) == false {
		return errors.New("Authentication canceled.")
	}

	token, err := PollToken(code)

	if err != nil {
		xbmc.Notify("Quasar", err.Error(), config.AddonIcon())
		return err
	}

	success := "Woohoo!"
	if fromSettings {
		success += " (Save your settings!)"
	}
	xbmc.Notify("Quasar", success, config.AddonIcon())
	xbmc.SetSetting("trakt_token", token.AccessToken)
	xbmc.SetSetting("trakt_refresh_token", token.RefreshToken)
	return nil
}

func Authorized() error {
	if config.Get().TraktToken == "" {
		err := Authorize(false)
		if err != nil {
			return err
		}
	}
	return nil
}

func AddToWatchlist(itemType string, tmdbId string) (resp *napping.Response, err error) {
	if err := Authorized(); err != nil {
		return nil, err
	}

	endPoint := "sync/watchlist"
	return Post(endPoint, bytes.NewBufferString(fmt.Sprintf(`{"%s": [{"ids": {"tmdb": %s}}]}`, itemType, tmdbId)))
}

func RemoveFromWatchlist(itemType string, tmdbId string) (resp *napping.Response, err error) {
	if err := Authorized(); err != nil {
		return nil, err
	}

	endPoint := "sync/watchlist/remove"
	return Post(endPoint, bytes.NewBufferString(fmt.Sprintf(`{"%s": [{"ids": {"tmdb": %s}}]}`, itemType, tmdbId)))
}

func AddToCollection(itemType string, tmdbId string) (resp *napping.Response, err error) {
	if err := Authorized(); err != nil {
		return nil, err
	}

	endPoint := "sync/collection"
	return Post(endPoint, bytes.NewBufferString(fmt.Sprintf(`{"%s": [{"ids": {"tmdb": %s}}]}`, itemType, tmdbId)))
}

func RemoveFromCollection(itemType string, tmdbId string) (resp *napping.Response, err error) {
	if err := Authorized(); err != nil {
		return nil, err
	}

	endPoint := "sync/collection/remove"
	return Post(endPoint, bytes.NewBufferString(fmt.Sprintf(`{"%s": [{"ids": {"tmdb": %s}}]}`, itemType, tmdbId)))
}

func Scrobble(action string, contentType string, tmdbId int, runtime int) {
	if err := Authorized(); err != nil {
		return
	}
	if action != "update" {
		log.Notice(action, contentType, tmdbId)
	}

	retVal := xbmc.GetWatchTimes()

	errStr := retVal["error"]
	watchedTime, _ := strconv.ParseFloat(retVal["watchedTime"], 64)
	videoDuration, _ := strconv.ParseFloat(retVal["videoDuration"], 64)
	if errStr != "" {
		log.Warning(errStr)
	} else {
		scrobbleTime = watchedTime
		scrobbleEnd = videoDuration
	}
	if action == "update" {
		return
	}

	if scrobbleEnd == 0 {
		if runtime != 0 {
			scrobbleEnd = float64(runtime)
			log.Warningf("Using specified runtime of %d", runtime)
		} else {
			if contentType == "movie" {
				scrobbleEnd = 7200
			} else {
				scrobbleEnd = 2700
			}
			log.Warningf("Using fallback runtime of %d", videoDuration)
		}
	}

	progress := scrobbleTime / math.Floor(scrobbleEnd) * 100

	log.Infof("Progress: %f%%, watched: %fs, duration: %fs", progress, scrobbleTime, scrobbleEnd)

	if action == "stop" {
		scrobbleTime = 0
		scrobbleEnd = 0
	}

	endPoint := fmt.Sprintf("scrobble/%s", action)
	resp, err := Post(endPoint, bytes.NewBufferString(fmt.Sprintf(`{"%s": {"ids": {"tmdb": %d}}, "progress": %f}`, contentType, tmdbId, progress)))
	if err != nil {
		log.Error(err.Error())
		xbmc.Notify("Quasar", "Scrobble failed, check your logs.", config.AddonIcon())
	} else if resp.Status() != 201 {
		log.Errorf("Failed to scrobble %s #%d to %s at %f: %d", contentType, tmdbId, action, progress, resp.Status())
	}
}
