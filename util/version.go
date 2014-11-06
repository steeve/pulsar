package util

import "fmt"

var (
	GitCommit string
	Version   string
)

func UserAgent() string {
	return fmt.Sprintf("Pulsar/%s", Version)
}
