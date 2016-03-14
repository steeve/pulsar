package api

import (
	"io"
	"os"
	"fmt"
	"strconv"
	"strings"
	"net/url"
	"net/http"
	"compress/gzip"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/osdb"
	"github.com/scakemyer/quasar/util"
	"github.com/scakemyer/quasar/xbmc"
)

var subLog = logging.MustGetLogger("subtitles")

func appendLocalFilePayloads(playingFile string, payloads *[]osdb.SearchPayload) error {
	file, err := os.Open(playingFile)
	if err != nil {
		return err
	}
	defer file.Close()

	hashPayload := osdb.SearchPayload{}
	if h, err := osdb.HashFile(file); err == nil {
		hashPayload.Hash = h
	}
	if s, err := file.Stat(); err == nil {
		hashPayload.Size = s.Size()
	}

	*payloads = append(*payloads, []osdb.SearchPayload{
		hashPayload,
		{Query: filepath.Base(playingFile)},
	}...)

	return nil
}

func appendMoviePayloads(labels map[string]string, payloads *[]osdb.SearchPayload) error {
	title := labels["VideoPlayer.OriginalTitle"]
	if title == "" {
		title = labels["VideoPlayer.Title"]
	}
	*payloads = append(*payloads, osdb.SearchPayload{
		Query: fmt.Sprintf("%s %s", title, labels["VideoPlayer.Year"]),
	})
	return nil
}

func appendEpisodePayloads(labels map[string]string, payloads *[]osdb.SearchPayload) error {
	season := -1
	if labels["VideoPlayer.Season"] != "" {
		if s, err := strconv.Atoi(labels["VideoPlayer.Season"]); err == nil {
			season = s
		}
	}
	episode := -1
	if labels["VideoPlayer.Episode"] != "" {
		if e, err := strconv.Atoi(labels["VideoPlayer.Episode"]); err == nil {
			episode = e
		}
	}
	if season >= 0 && episode > 0 {
		searchString := fmt.Sprintf("%s S%02dE%02d", labels["VideoPlayer.TVshowtitle"], season, episode)
		*payloads = append(*payloads, osdb.SearchPayload{
			Query: searchString,
		})
	}
	return nil
}

func SubtitlesIndex(ctx *gin.Context) {
	q := ctx.Request.URL.Query()
	searchString := q.Get("searchstring")
	languages := strings.Split(q.Get("languages"), ",")

	labels := xbmc.InfoLabels(
		"VideoPlayer.Title",
		"VideoPlayer.OriginalTitle",
		"VideoPlayer.Year",
		"VideoPlayer.TVshowtitle",
		"VideoPlayer.Season",
		"VideoPlayer.Episode",
	)
	playingFile := xbmc.PlayerGetPlayingFile()

	// are we reading a file from Quasar?
	if strings.HasPrefix(playingFile, util.GetHTTPHost()) {
		playingFile = strings.Replace(playingFile, util.GetHTTPHost() + "/files", config.Get().DownloadPath, 1)
		playingFile, _ = url.QueryUnescape(playingFile)
	}

	for i, lang := range languages {
		if lang == "Portuguese (Brazil)" {
			languages[i] = "pob"
		} else {
			isoLang := xbmc.ConvertLanguage(lang, xbmc.ISO_639_2)
			if isoLang == "gre" {
				isoLang = "ell"
			}
			languages[i] = isoLang
		}
	}

	payloads := []osdb.SearchPayload{}
	if searchString != "" {
		payloads = append(payloads, osdb.SearchPayload{
			Query:     searchString,
			Languages: strings.Join(languages, ","),
		})
	} else {
		if strings.HasPrefix(playingFile, "http://") == false && strings.HasPrefix(playingFile, "https://") == false {
			appendLocalFilePayloads(playingFile, &payloads)
		}

		if labels["VideoPlayer.TVshowtitle"] != "" {
			appendEpisodePayloads(labels, &payloads)
		} else {
			appendMoviePayloads(labels, &payloads)
		}
	}

	for i, payload := range payloads {
		payload.Languages = strings.Join(languages, ",")
		payloads[i] = payload
	}

	client, err := osdb.NewClient()
	if err != nil {
		ctx.AbortWithError(404, err)
		return
	}
	if err := client.LogIn("", "", ""); err != nil {
		ctx.AbortWithError(404, err)
		return
	}

	items := make(xbmc.ListItems, 0)
	results, _ := client.SearchSubtitles(payloads)
	for _, sub := range results {
		rating, _ := strconv.ParseFloat(sub.SubRating, 64)
		subLang := sub.LanguageName
		if subLang == "Brazilian" {
			subLang = "Portuguese (Brazil)"
		}
		item := &xbmc.ListItem{
			Label:     subLang,
			Label2:    sub.SubFileName,
			Icon:      strconv.Itoa(int((rating / 2) + 0.5)),
			Thumbnail: sub.ISO639,
			Path: UrlQuery(UrlForXBMC("/subtitle/%s", sub.IDSubtitleFile),
				"file", sub.SubFileName,
				"lang", sub.SubLanguageID,
				"fmt", sub.SubFormat,
				"dl", sub.SubDownloadLink),
			Properties: make(map[string]string),
		}
		if sub.MatchedBy == "moviehash" {
			item.Properties["sync"] = "true"
		}
		if sub.SubHearingImpaired == "1" {
			item.Properties["hearing_imp"] = "true"
		}
		items = append(items, item)
	}

	ctx.JSON(200, xbmc.NewView("", items))
}

func SubtitleGet(ctx *gin.Context) {
	q := ctx.Request.URL.Query()
	file := q.Get("file")
	dl := q.Get("dl")

	resp, err := http.Get(dl)
	if err != nil {
		ctx.AbortWithError(404, err)
		return
	}
	defer resp.Body.Close()

	reader, err := gzip.NewReader(resp.Body)
	if err != nil {
		ctx.AbortWithError(404, err)
		return
	}
	defer reader.Close()

	subtitlesPath := filepath.Join(config.Get().DownloadPath, "Subtitles")
	if _, err := os.Stat(subtitlesPath); os.IsNotExist(err) {
		if err := os.Mkdir(subtitlesPath, 0755); err != nil{
			subLog.Error("Unable to create Subtitles folder")
		}
	}

	outFile, err := os.Create(filepath.Join(subtitlesPath, file))
	if err != nil {
		ctx.AbortWithError(404, err)
		return
	}
	defer outFile.Close()

	io.Copy(outFile, reader)

	ctx.JSON(200, xbmc.NewView("", xbmc.ListItems{
		{Label: file, Path: outFile.Name()},
	}))
}
