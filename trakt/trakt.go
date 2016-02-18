package trakt

import (
	"fmt"
	"net/url"
	"net/http"

	"github.com/jmcvetta/napping"
)

const (
	ApiUrl      = "https://api-v2launch.trakt.tv"
	ClientId    = "4407ab20a3a971e7c92d4996b36b76d0312ea085cb139d7c38a1a4c9f8428f60"
	ApiVersion  = "2"
	Limit       = "20"
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

func Get(endPoint string, params url.Values) (resp *napping.Response, err error) {
	header := http.Header{
		"Content-type": []string{"application/json"},
		"trakt-api-key": []string{ClientId},
		"trakt-api-version": []string{ApiVersion},
	}

	req := napping.Request{
		Url: fmt.Sprintf("%s/%s", ApiUrl, endPoint),
		Method: "GET",
		Params: &params,
		Header: &header,
	}

	return napping.Send(&req)
}
