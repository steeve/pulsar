package providers

import (
	"github.com/scakemyer/pulsar/bittorrent"
	"github.com/scakemyer/pulsar/tmdb"
	"github.com/scakemyer/pulsar/tvdb"
)

type Searcher interface {
	SearchLinks(query string) []*bittorrent.Torrent
}

type MovieSearcher interface {
	SearchMovieLinks(movie *tmdb.Movie) []*bittorrent.Torrent
}

type EpisodeSearcher interface {
	SearchEpisodeLinks(show *tvdb.Show, episode *tvdb.Episode) []*bittorrent.Torrent
}
