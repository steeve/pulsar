package providers

import (
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/tmdb"
	"github.com/steeve/pulsar/tvdb"
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
