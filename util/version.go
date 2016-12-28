package util

import "fmt"

var (
	Version   string
)

func UserAgent() string {
	return fmt.Sprintf("Quasar/%s libtorrent/%s", Version[1:len(Version) - 1], "v1.1.1")
}
