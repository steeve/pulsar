package ga

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/nu7hatch/gouuid"
	"github.com/i96751414/pulsar/cache"
	"github.com/i96751414/pulsar/config"
	"github.com/i96751414/pulsar/util"
)

const (
	trackingID        = "UA-40799149-7"
	gaEndpoint        = "https://www.google-analytics.com/collect?%s"
	clientIdCacheTime = 100 * 365 * 24 * time.Hour // 100 years
)

var (
	httpClient = &http.Client{}
)

func getClientId() string {
	clientId := ""
	key := "io.steeve.pulsar.ga"
	cacheStore := cache.NewFileStore(path.Join(config.Get().ProfilePath))
	if err := cacheStore.Get(key, &clientId); err != nil {
		clientUUID, _ := uuid.NewV4()
		clientId := clientUUID.String()
		cacheStore.Set(key, clientId, clientIdCacheTime)
	}
	return clientId
}

func track(payload url.Values) {
	payload.Set("v", "1")
	payload.Set("tid", trackingID)
	payload.Set("cid", getClientId())
	payload.Set("aip", "1")

	req, err := http.NewRequest("HEAD", fmt.Sprintf(gaEndpoint, payload.Encode()), nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", util.UserAgent())
	httpClient.Do(req)
}

func TrackPageView(path string) {
	payload := url.Values{}
	payload.Set("t", "pageview")
	payload.Set("dp", path)
	track(payload)
}

func TrackEvent(category string, action string, label string, value int) {
	payload := url.Values{}
	payload.Set("t", "event")
	payload.Set("ec", category)
	payload.Set("ea", action)
	payload.Set("el", label)
	if value >= 0 {
		payload.Set("ev", strconv.Itoa(value))
	}
	track(payload)
}

func TrackSocial(action string, network string, target string) {
	payload := url.Values{}
	payload.Set("t", "social")
	payload.Set("sa", action)
	payload.Set("sn", network)
	payload.Set("st", target)
	track(payload)
}

func TrackException(description string, isFatal bool) {
	payload := url.Values{}
	payload.Set("t", "exception")
	payload.Set("exd", description)
	if isFatal {
		payload.Set("exf", "1")
	} else {
		payload.Set("exf", "0")
	}
	track(payload)

}

func TrackTiming(category string, variable string, time int, label string) {
	payload := url.Values{}
	payload.Set("t", "timing")
	payload.Set("utc", category)
	payload.Set("utv", variable)
	payload.Set("utt", strconv.Itoa(time))
	payload.Set("utl", label)
	track(payload)
}
