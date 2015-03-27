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

var (
	paramsRe = regexp.MustCompile("ytplayer.config = ({.*?});")
)

type YTPlayerConfig struct {
	Args struct {
		FmtList                string `json:"fmt_list"`
		UrlEncodedFmtStreamMap string `json:"url_encoded_fmt_stream_map"`
	} `json:"args"`
}

func Resolve(youtubeId string) ([]string, error) {
	resp, err := http.Get(fmt.Sprintf("http://www.youtube.com/watch?v=%s&gl=US&hl=en&has_verified=1&bpctr=9999999999", youtubeId))
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
