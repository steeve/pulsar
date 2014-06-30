package util

import (
	"math"

	"github.com/steeve/pulsar/bittorrent"
)

const (
	CoefRipType = 10.0
)

type ByQuality []*bittorrent.Torrent

func QualityFactor(t *bittorrent.Torrent) float64 {
	result := float64(t.Seeds)
	if t.Resolution > bittorrent.ResolutionUnkown {
		result *= math.Pow(float64(t.Resolution), 3)
	}
	if t.RipType > bittorrent.RipUnknown {
		result *= float64(t.RipType)
	}
	return result
}

func (a ByQuality) Len() int {
	return len(a)
}

func (a ByQuality) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a ByQuality) Less(i, j int) bool {
	return QualityFactor(a[i]) < QualityFactor(a[j])
}
