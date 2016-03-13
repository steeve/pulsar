package cloudhole

import (
	"fmt"
	"net/http"
	"math/rand"

	"github.com/jmcvetta/napping"
)

var clearances []*Clearance
var failed     bool

type Clearance struct {
	Id        string `json:"_id"`
	Date      string `json:"createDate"`
	UserAgent string `json:"userAgent"`
	Cookies   string `json:"cookies"`
	Label     string `json:"label"`
}

func GetClearance() (clearance *Clearance) {
	if len(clearances) > 0 {
		clearance = clearances[rand.Intn(len(clearances))]
		return clearance
	}

	header := http.Header{
		"Content-type": []string{"application/json"},
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
	}

	return clearance
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

	if err == nil && resp.Status() == 200 {
		resp.Unmarshal(&clearances)
	}
}
