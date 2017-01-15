package cloudhole

import (
	"fmt"
	"errors"
	"net/http"
	"math/rand"

	"github.com/jmcvetta/napping"
	"github.com/scakemyer/quasar/xbmc"
	"github.com/scakemyer/quasar/config"
)

var (
	apiKey           = ""
	clearances         []*Clearance
	defaultClearance = &Clearance{
		UserAgent: "Mozilla/5.0 (X11; NetBSD amd64; rv:42.0) Gecko/20100101 Firefox/42.0",
	}
)

type ApiKey struct {
	Key       string `json:"key"`
}

type Clearance struct {
	Id        string `json:"_id"`
	Key       string `json:"key"`
	Date      string `json:"createDate"`
	UserAgent string `json:"userAgent"`
	Cookies   string `json:"cookies"`
	Label     string `json:"label"`
}

func ResetClearances() {
	apiKey     = ""
	clearances = []*Clearance{}
	xbmc.SetSetting("cloudhole_key", "")
}

func GetClearance() (clearance *Clearance, err error) {
	if len(clearances) > 0 {
		clearance = clearances[rand.Intn(len(clearances))]
		return clearance, nil
	}

	apiKey := config.Get().CloudHoleKey

	// Get our CloudHole key if not specified
	if apiKey == "" {
		header := http.Header{
			"Content-type": []string{"application/json"},
		}
		params := napping.Params{}.AsUrlValues()

		req := napping.Request{
			Url: fmt.Sprintf("%s/%s", "https://cloudhole.herokuapp.com", "key"),
			Method: "GET",
			Params: &params,
			Header: &header,
		}

		resp, err := napping.Send(&req)

		if err == nil && resp.Status() == 200 {
			newKey := &ApiKey{ Key: "" }
			resp.Unmarshal(&newKey)
			apiKey = newKey.Key
			xbmc.SetSetting("cloudhole_key", apiKey)
		}
	}

	// Still empty, return default clearance
	if apiKey == "" {
		return defaultClearance, nil
	}

	header := http.Header{
		"Content-type": []string{"application/json"},
		"Authorization": []string{apiKey},
	}
	params := napping.Params{}.AsUrlValues()

	req := napping.Request{
		Url: fmt.Sprintf("%s/%s", "https://cloudhole.herokuapp.com", "clearances"),
		Method: "GET",
		Params: &params,
		Header: &header,
	}

	resp, err := napping.Send(&req)

	if err == nil && resp.Status() == 200 {
		resp.Unmarshal(&clearances)
	} else if resp.Status() == 503 {
		GetSurgeClearances()
	}

	if len(clearances) > 0 {
		clearance = clearances[rand.Intn(len(clearances))]
	} else {
		err = errors.New("Failed to get new clearance.")
		clearance = defaultClearance
		clearances = append(clearances, defaultClearance)
	}

	return clearance, err
}

func GetSurgeClearances() {
	header := http.Header{
		"Content-type": []string{"application/json"},
	}
	params := napping.Params{}.AsUrlValues()

	req := napping.Request{
		Url: fmt.Sprintf("%s/%s", "https://cloudhole.surge.sh", "cloudhole.json"),
		Method: "GET",
		Params: &params,
		Header: &header,
	}

	resp, err := napping.Send(&req)

	var tmpClearances []*Clearance
	if err == nil && resp.Status() == 200 {
		resp.Unmarshal(&tmpClearances)
	}

	apiKey := config.Get().CloudHoleKey
	for _, clearance := range tmpClearances {
		if clearance.Key == apiKey {
			clearances = append(clearances, clearance)
		}
	}
}
