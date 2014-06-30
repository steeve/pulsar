package bittorrent

import (
	"encoding/base32"
	"encoding/hex"
	"net/url"
	"strings"
)

type Magnet struct {
	uri         string
	InfoHash    string
	DisplayName string
	Trackers    []string
}

func NewMagnet(magnetURI string) (magnet *Magnet, err error) {
	uri, err := url.Parse(magnetURI)
	if err != nil {
		return
	}
	vals := uri.Query()
	hash := strings.ToUpper(strings.TrimPrefix(vals.Get("xt"), "urn:btih:"))

	// for backward compatibility
	unBase32Hash, err := base32.StdEncoding.DecodeString(hash)
	if err == nil {
		hash = hex.EncodeToString(unBase32Hash)
	}

	trackers := make([]string, len(vals["tr"]))
	for i, tracker := range vals["tr"] {
		trackers[i] = strings.Replace(tracker, "\\", "", -1)
	}

	magnet = &Magnet{
		uri:         magnetURI,
		InfoHash:    strings.ToLower(hash),
		DisplayName: vals.Get("dn"),
		Trackers:    trackers,
	}
	return
}

func (m *Magnet) String() string {
	return m.uri
}
