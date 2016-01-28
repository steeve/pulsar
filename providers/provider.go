package providers

import (
	"github.com/scakemyer/quasar/bittorrent"
	"github.com/scakemyer/quasar/tmdb"
	"github.com/scakemyer/quasar/tvdb"
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
