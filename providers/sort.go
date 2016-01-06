package providers

import (
	"math"

	"github.com/i96751414/pulsar/bittorrent"
)

type ByResolution []*bittorrent.Torrent

func (a ByResolution) Len() int           { return len(a) }
func (a ByResolution) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByResolution) Less(i, j int) bool { return a[i].Resolution < a[j].Resolution }

type BySeeds []*bittorrent.Torrent

func (a BySeeds) Len() int           { return len(a) }
func (a BySeeds) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a BySeeds) Less(i, j int) bool { return a[i].Seeds < a[j].Seeds }

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
