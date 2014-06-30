package trakt

const (
	ENDPOINT    = "http://api.trakt.tv"
	APIKEY      = "480555541c12a378b1aac15054a95698"
	SearchLimit = "300"
)

type Ratings struct {
	Percentage int `json:"percentage"`
	Votes      int `json:"votes"`
	Loved      int `json:"loved"`
	Hated      int `json:"hated"`
}

type Images struct {
	Poster string `json:"poster"`
	FanArt string `json:"fanart"`
	Banner string `json:"banner"`
	Screen string `json:"screen"`
}
