package providers

import (
	"github.com/steeve/pulsar/tmdb"
	"github.com/steeve/pulsar/trakt"
)

type MovieSearcher interface {
	SearchMovieLinks(movie *tmdb.Movie) []string
}

type EpisodeSearcher interface {
	SearchEpisodeLinks(episode *trakt.ShowEpisode) []string
}
