package bittorrent

import (
	"crypto/sha1"
	"crypto/tls"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/scakemyer/quasar/xbmc"
	"github.com/zeebo/bencode"
)

type Torrent struct {
	URI       string   `json:"uri"`
	InfoHash  string   `json:"info_hash"`
	Name      string   `json:"name"`
	Trackers  []string `json:"trackers"`
	Size      string   `json:"size"`
	Seeds     int64    `json:"seeds"`
	Peers     int64    `json:"peers"`
	IsPrivate bool     `json:"is_private"`
	Provider  string   `json:"provider"`
	Icon      string   `json:"icon"`
	Multi     bool

	Resolution  int    `json:"resolution"`
	VideoCodec  int    `json:"video_codec"`
	AudioCodec  int    `json:"audio_codec"`
	Language    string `json:"language"`
	RipType     int    `json:"rip_type"`
	SceneRating int    `json:"scene_rating"`

	hasResolved bool
}

const (
	ResolutionUnknown = iota
	Resolution480p
	Resolution720p
	Resolution1080p
	Resolution1440p
	Resolution4k2k
)

var (
	resolutionTags = map[*regexp.Regexp]int{
		regexp.MustCompile(`\W+(480p|xvid|dvd|hdtv)\W*`): Resolution480p,
		regexp.MustCompile(`\W+(720p|hdrip)\W*`):         Resolution720p,
		regexp.MustCompile(`\W+1080p\W*`):                Resolution1080p,
	}
	Resolutions = []string{"", "480p", "720p", "1080p"}
	Colors = []string{"", "FFA56F01", "FF539A02", "FF0166FC"}
)

const (
	RipUnknown = iota
	RipCam
	RipTS
	RipTC
	RipScr
	RipDVDScr
	RipDVD
	RipHDTV
	RipWeb
	RipBluRay
)

var (
	ripTags = map[*regexp.Regexp]int{
		regexp.MustCompile(`\W+(cam|camrip|hdcam)\W*`):   RipCam,
		regexp.MustCompile(`\W+(ts|telesync)\W*`):        RipTS,
		regexp.MustCompile(`\W+(tc|telecine)\W*`):        RipTC,
		regexp.MustCompile(`\W+(scr|screener)\W*`):       RipScr,
		regexp.MustCompile(`\W+dvd\W*scr\W*`):            RipDVDScr,
		regexp.MustCompile(`\W+dvd\W*rip\W*`):            RipDVD,
		regexp.MustCompile(`\W+hd(tv|rip)\W*`):           RipHDTV,
		regexp.MustCompile(`\W+(web\W*dl|web\W*rip)\W*`): RipWeb,
		regexp.MustCompile(`\W+(bluray|b[rd]rip)\W*`):    RipBluRay,
	}
	Rips = []string{"", "Cam", "TeleSync", "TeleCine", "Screener", "DVD Screener", "DVDRip", "HDTV", "WebDL", "Blu-Ray"}
)

const (
	RatingUnkown = iota
	RatingProper
	RatingNuked
)

var (
	sceneTags = map[*regexp.Regexp]int{
		regexp.MustCompile(`\W+nuked\W*`):  RatingNuked,
		regexp.MustCompile(`\W+proper\W*`): RatingProper,
	}
)

const (
	CodecUnknown = iota

	CodecXVid
	CodecH264
	CodecH265

	CodecMp3
	CodecAAC
	CodecAC3
	CodecDTS
	CodecDTSHD
	CodecDTSHDMA
)

var (
	videoTags = map[*regexp.Regexp]int{
		regexp.MustCompile(`\W+([hx]264|1080p|hdrip)\W*`): CodecH264,
		regexp.MustCompile(`\W+([hx]265|hevc)\W*`):        CodecH265,
		regexp.MustCompile(`\W+xvid\W*`):                  CodecXVid,
	}
	audioTags = map[*regexp.Regexp]int{
		regexp.MustCompile(`\W+mp3\W*`):              CodecMp3,
		regexp.MustCompile(`\W+aac\W*`):              CodecAAC,
		regexp.MustCompile(`\W+(ac3|[Dd]*5\W+1)\W*`): CodecAC3,
		regexp.MustCompile(`\W+dts\W*`):              CodecDTS,
		regexp.MustCompile(`\W+dts\W+hd\W*`):         CodecDTSHD,
		regexp.MustCompile(`\W+dts\W+hd\W+ma\W*`):    CodecDTSHDMA,
	}
	Codecs = []string{"", "Xvid", "H.264", "H.265", "MP3", "AAC", "AC3", "DTS", "DTS HD", "DTS HD MA"}
)

var (
	httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
)

const (
	torCache = "http://torcache.net/torrent/%s.torrent"
)

// Used to avoid infinite recursion in UnmarshalJSON
type torrent Torrent

func (t *Torrent) UnmarshalJSON(b []byte) error {
	tmp := torrent{}
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}
	*t = Torrent(tmp)
	t.initialize()
	return nil
}

func (t *Torrent) initializeFromMagnet() {
	magnetURI, _ := url.Parse(t.URI)
	vals := magnetURI.Query()
	hash := strings.ToUpper(strings.TrimPrefix(vals.Get("xt"), "urn:btih:"))

	// for backward compatibility
	if unBase32Hash, err := base32.StdEncoding.DecodeString(hash); err == nil {
		hash = hex.EncodeToString(unBase32Hash)
	}

	if t.InfoHash == "" {
		t.InfoHash = strings.ToLower(hash)
	}
	if t.Name == "" {
		t.Name = vals.Get("dn")
	}

	if len(t.Trackers) == 0 {
		t.Trackers = make([]string, 0)
		for _, tracker := range vals["tr"] {
			t.Trackers = append(t.Trackers, strings.Replace(string(tracker), "\\", "", -1))
		}
	}
}

func (t *Torrent) Resolve() error {
	if t.IsMagnet() {
		t.hasResolved = true
		return nil
	}

	var torrentFile struct {
		Announce     string                 `bencode:"announce"`
		AnnounceList [][]string             `bencode:"announce-list"`
		Info         map[string]interface{} `bencode:"info"`
	}

	// We don't need trackers for public torrents since we'll find them on the
	// DHT or public trackers
	if (t.InfoHash != "" && t.Name != "" && t.Peers > 0 && t.Seeds > 0) && (t.IsPrivate == false || len(t.Trackers) > 0) {
		return nil
	}

	parts := strings.Split(t.URI, "|")
	uri := parts[0]
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return err
	}
	if len(parts) > 1 {
		for _, part := range parts[1:] {
			keyVal := strings.SplitN(part, "=", 2)
			req.Header.Add(keyVal[0], keyVal[1])
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	dec := bencode.NewDecoder(resp.Body)

	// FIXME!!!!
	if err := dec.Decode(&torrentFile); err != nil {
		return err
	}
	if t.InfoHash == "" {
		hasher := sha1.New()
		bencode.NewEncoder(hasher).Encode(torrentFile.Info)
		t.InfoHash = hex.EncodeToString(hasher.Sum(nil))
	}
	if t.Name == "" {
		t.Name = torrentFile.Info["name"].(string)
	}
	if len(t.Trackers) == 0 {
		t.Trackers = append(t.Trackers, torrentFile.Announce)
		for _, trackers := range torrentFile.AnnounceList {
			t.Trackers = append(t.Trackers, trackers...)
		}
	}

	t.hasResolved = true

	t.initialize()

	return nil
}

func (t *Torrent) initialize() {
	if strings.HasPrefix(t.URI, "magnet:") {
		t.initializeFromMagnet()
	}

	if t.Resolution == ResolutionUnknown {
		t.Resolution = matchTags(t, resolutionTags)
	}
	if t.VideoCodec == CodecUnknown {
		t.VideoCodec = matchTags(t, videoTags)
	}
	if t.AudioCodec == CodecUnknown {
		t.AudioCodec = matchTags(t, audioTags)
	}
	if t.RipType == RipUnknown {
		t.RipType = matchTags(t, ripTags)
	}
	if t.SceneRating == RatingUnkown {
		t.SceneRating = matchTags(t, sceneTags)
	}
}

func NewTorrent(uri string) *Torrent {
	t := &Torrent{
		URI: uri,
	}
	t.initialize()
	return t
}

func matchTags(t *Torrent, tokens map[*regexp.Regexp]int) int {
	lowName := strings.ToLower(t.Name)
	codec := 0
	for re, value := range tokens {
		if re.MatchString(lowName) {
			codec = value
		}
	}
	return codec
}

func (t *Torrent) IsMagnet() bool {
	return strings.HasPrefix(t.URI, "magnet:")
}

func (t *Torrent) Magnet() string {
	if t.hasResolved == false {
		t.Resolve()
	}
	if t.IsMagnet() {
		return t.URI + "&" + url.Values{"as": []string{fmt.Sprintf(torCache, t.InfoHash)}}.Encode()
	}
	params := url.Values{}
	params.Set("dn", t.Name)
	for _, tracker := range t.Trackers {
		params.Add("tr", tracker)
	}
	params.Add("as", t.URI)
	return fmt.Sprintf("magnet:?xt=urn:btih:%s&%s", t.InfoHash, params.Encode())
}

func (t *Torrent) StreamInfo() *xbmc.StreamInfo {
	sie := &xbmc.StreamInfo{
		Video: &xbmc.StreamInfoEntry{
			Codec: Codecs[t.VideoCodec],
		},
		Audio: &xbmc.StreamInfoEntry{
			Codec: Codecs[t.AudioCodec],
		},
	}

	switch t.Resolution {
	case Resolution480p:
		sie.Video.Width = 853
		sie.Video.Height = 480
		break
	case Resolution720p:
		sie.Video.Width = 1280
		sie.Video.Height = 720
		break
	case Resolution1080p:
		sie.Video.Width = 1920
		sie.Video.Height = 1080
		break
	}

	return sie
}
