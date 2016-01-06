package providers

import (
	"github.com/i96751414/pulsar/bittorrent"
	"github.com/i96751414/pulsar/tmdb"
	"github.com/i96751414/pulsar/tvdb"
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
