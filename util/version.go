package util

import (
	"fmt"

	"github.com/steeve/libtorrent-go"
)

var (
	GitCommit string
	Version   string
)

func UserAgent() string {
	return fmt.Sprintf("Pulsar/%s libtorrent/%s", Version, libtorrent.LIBTORRENT_VERSION)
}
