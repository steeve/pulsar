package tvdb

import (
	"fmt"
	"path"
	"sort"
	"time"
	"bytes"
	"strconv"
	"net/http"
	"io/ioutil"
	"archive/zip"
	"encoding/xml"

	"github.com/scakemyer/quasar/cache"
	"github.com/scakemyer/quasar/config"
)

const (
	tvdbUrl                 = "http://thetvdb.com"
	tvdbEndpoint            = tvdbUrl + "/api"
	apiKey                  = "1D62F2F90030C444"
	burstRate               = 30
	burstTime               = 1 * time.Second
	simultaneousConnections = 20
	cacheTime               = 2 * time.Hour
)

type SeasonList []*Season
type EpisodeList []*Episode

type Episode struct {
	Id            string `xml:"id"`
	Director      string `xml:"Director"`
	EpisodeName   string `xml:"EpisodeName"`
	EpisodeNumber int    `xml:"EpisodeNumber"`
	FirstAired    string `xml:"FirstAired"`
	GuestStars    string `xml:"GuestStars"`
	ImdbId        string `xml:"IMDB_ID"`
	Language      string `xml:"Language"`
	Overview      string `xml:"Overview"`
	Rating        string `xml:"Rating"`
	RatingCount   string `xml:"RatingCount"`
	SeasonNumber  int    `xml:"SeasonNumber"`
	Writer        string `xml:"Writer"`
	FileName      string `xml:"filename"`
	LastUpdated   string `xml:"lastupdated"`
	SeasonId      string `xml:"seasonid"`
	SeriesId      string `xml:"seriesid"`
	ThumbHeight   string `xml:"thumb_height"`
	ThumbWidth    string `xml:"thumb_width"`

	AbsoluteNumber       int    `xml:"-"`
	AbsoluteNumberString string `xml:"absolute_number"`
}

type Show struct {
	Id            int    `xml:"id"`
	ActorsSimple  string `xml:"Actors"`
	AirsDayOfWeek string `xml:"Airs_DayOfWeek"`
	AirsTime      string `xml:"Airs_Time"`
	ContentRating string `xml:"ContentRating"`
	FirstAired    string `xml:"FirstAired"`
	Genre         string `xml:"Genre"`
	ImdbId        string `xml:"IMDB_ID"`
	Language      string `xml:"Language"`
	Network       string `xml:"Network"`
	NetworkId     string `xml:"NetworkID"`
	Overview      string `xml:"Overview"`
	Rating        string `xml:"Rating"`
	RatingCount   string `xml:"RatingCount"`
	RuntimeString string `xml:"Runtime"`
	SeriesID      string `xml:"SeriesID"`
	SeriesName    string `xml:"SeriesName"`
	Status        string `xml:"Status"`
	Banner        string `xml:"banner"`
	FanArt        string `xml:"fanart"`
	LastUpdated   int    `xml:"lastupdated"`
	Poster        string `xml:"poster"`

	Runtime int `xml:"-"`

	Seasons SeasonList `xml:"-"`
	Banners []*Banner  `xml:"-"`
	Actors  []*Actor   `xml:"-"`
}

type Season struct {
	Season   int
	Episodes EpisodeList
}

type Banner struct {
	Id            string `xml:"id"`
	BannerPath    string `xml:"BannerPath"`
	BannerType    string `xml:"BannerType"`
	BannerType2   string `xml:"BannerType2"`
	Colors        string `xml:"Colors"`
	Language      string `xml:"Language"`
	Rating        string `xml:"Rating"`
	RatingCount   int    `xml:"RatingCount"`
	SeriesName    string `xml:"SeriesName"`
	ThumbnailPath string `xml:"ThumbnailPath"`
	VignettePath  string `xml:"VignettePath"`
	Season        int    `xml:"Season,omitempty"`
}

type Actor struct {
	Id        string `xml:"id"`
	Image     string `xml:"Image"`
	Name      string `xml:"Name"`
	Role      string `xml:"Role"`
	SortOrder int    `xml:"SortOrder"`
}

func getShow(tvdbId int, language string) (*Show, error) {
	var serie struct {
		Serie    *Show      `xml:"Series"`
		Episodes []*Episode `xml:"Episode"`
	}
	var banners struct {
		Banners []*Banner `xml:"Banner"`
	}
	var actors struct {
		Actors []*Actor `xml:"Actor"`
	}

	resp, err := http.Get(fmt.Sprintf("%s/%s/series/%d/all/%s.zip", tvdbEndpoint, apiKey, tvdbId, language))
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	br := bytes.NewReader(b)
	zipReader, err := zip.NewReader(br, int64(br.Len()))
	if err != nil {
		return nil, err
	}
	for _, file := range zipReader.File {
		f, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer f.Close()
		decoder := xml.NewDecoder(f)
		switch file.Name {
		case language + ".xml":
			if err := decoder.Decode(&serie); err != nil {
				return nil, err
			}
		case "banners.xml":
			if err := decoder.Decode(&banners); err != nil {
				return nil, err
			}
		case "actors.xml":
			if err := decoder.Decode(&actors); err != nil {
				return nil, err
			}
		}
	}

	show := serie.Serie
	show.Actors = actors.Actors
	show.Banners = banners.Banners
	show.Seasons = make([]*Season, 0)

	sort.Sort(sort.Reverse(BannersByRating(banners.Banners)))
	sort.Sort(BySeasonAndEpisodeNumber(serie.Episodes))

	if rt, err := strconv.Atoi(show.RuntimeString); err == nil {
		show.Runtime = rt
	}

	curSeasonNumber := -1
	for _, episode := range serie.Episodes {
		for _ = 0; curSeasonNumber < episode.SeasonNumber; curSeasonNumber++ {
			show.Seasons = append(show.Seasons, &Season{
				Season:   episode.SeasonNumber,
				Episodes: make([]*Episode, 0),
			})
		}
		season := show.Seasons[curSeasonNumber]
		if an, err := strconv.Atoi(episode.AbsoluteNumberString); err == nil {
			episode.AbsoluteNumber = an
		}
		season.Episodes = append(season.Episodes, episode)
	}

	return show, nil
}

func GetShow(tvdbId int, language string) (*Show, error) {
	var show *Show
	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath, "cache"))
	key := fmt.Sprintf("com.tvdb.show.%d.%s", tvdbId, language)
	if err := cacheStore.Get(key, &show); err != nil {
		newShow, err := getShow(tvdbId, language)
		if err != nil {
			return nil, err
		}
		if newShow != nil {
			cacheStore.Set(key, newShow, cacheTime)
		}
		show = newShow
	}
	return show, nil
}

type BySeasonAndEpisodeNumber []*Episode

func (a BySeasonAndEpisodeNumber) Len() int      { return len(a) }
func (a BySeasonAndEpisodeNumber) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BySeasonAndEpisodeNumber) Less(i, j int) bool {
	return (a[i].SeasonNumber*1000)+a[i].EpisodeNumber < (a[j].SeasonNumber*1000)+a[j].EpisodeNumber
}

type BannersByRating []*Banner

func (a BannersByRating) Len() int      { return len(a) }
func (a BannersByRating) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BannersByRating) Less(i, j int) bool {
	iRating, _ := strconv.ParseFloat(a[i].Rating, 32)
	jRating, _ := strconv.ParseFloat(a[j].Rating, 32)
	return iRating < jRating
}

func (s SeasonList) Len() int           { return len(s) }
func (s SeasonList) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s SeasonList) Less(i, j int) bool { return s[i].Season < s[j].Season }
