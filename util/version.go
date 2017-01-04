package util

import (
	"fmt"
	"github.com/scakemyer/libtorrent-go"
)

var (
	Version   string
)

func UserAgent() string {
	return fmt.Sprintf("Quasar/%s libtorrent/%s", Version[1:len(Version) - 1], libtorrent.Version())
}
