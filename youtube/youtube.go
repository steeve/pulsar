package youtube

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	youtubeKey = ""
	searchLink = "https://www.googleapis.com/youtube/v3/search?part=snippet&type=video&maxResults=5&q=%s&key=%s"
	watchLink  = "http://www.youtube.com/watch?v=%s&gl=US&hl=en&has_verified=1&bpctr=9999999999"
)

var (
	paramsRe = regexp.MustCompile("ytplayer.config = ({.*?});")
)

type YTPlayerConfig struct {
	Args struct {
		FmtList                string `json:"fmt_list"`
		UrlEncodedFmtStreamMap string `json:"url_encoded_fmt_stream_map"`
	} `json:"args"`
}

type YTSearchItems struct {
    Item[]struct{
    	ID struct{
    		Kind    string `json:"kind"`
    		VideoID string `json:"videoId"`
    	} `json:"id"`
    } `json:"items"`
}

func Search(name string) (string, error){
    url := fmt.Sprintf(searchLink, url.QueryEscape(name + " trailer"), youtubeKey)

 	r, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()

	items := YTSearchItems{}
	if err = json.NewDecoder(r.Body).Decode(&items); err != nil {
		return "", err
	}

	for _, item := range items.Item {
		if item.ID.VideoID != "" && item.ID.Kind == "youtube#video" {
			return item.ID.VideoID, nil
		}
	}

    return "", fmt.Errorf("Unable to find youtube#video !")
}

func Resolve(youtubeId string) ([]string, error) {
	resp, err := http.Get(fmt.Sprintf(watchLink, youtubeId))
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	matches := paramsRe.FindSubmatch(body)
	if len(matches) == 0 {
		return nil, fmt.Errorf("Unable to find player config !")
	}

	cfg := YTPlayerConfig{}
	if json.Unmarshal(matches[1], &cfg) != nil {
		return nil, err
	}

	streams := make([]string, 0)
	for _, stream := range strings.Split(cfg.Args.UrlEncodedFmtStreamMap, ",") {
		v, err := url.ParseQuery(stream)
		if err != nil {
			return nil, err
		}
		streams = append(streams, v.Get("url"))
	}
	return streams, nil
}
