package api

import (
	"fmt"

	"github.com/gorilla/mux"
	"github.com/steeve/pulsar/bittorrent"
)

var routes *mux.Router

const (
	PREFIX = "plugin://plugin.video.pulsar"
)

func Routes(btService *bittorrent.BTService) *mux.Router {
	if routes == nil {
		router := mux.NewRouter()

		router.HandleFunc("/", Index)

		router.HandleFunc("/movies/search", SearchMovies).Name("search_movies")
		router.HandleFunc("/movies/popular", PopularMovies).Name("popular_movies")
		router.HandleFunc("/movies/popular/{genre}", PopularMovies).Name("popular_movies_by_genre")
		router.HandleFunc("/movies/genres", MovieGenres).Name("movie_genres")
		router.HandleFunc("/movies/{imdbId}/links", MovieLinks).Name("movie_links")
		router.HandleFunc("/movies/{imdbId}/play", MoviePlay).Name("movie_play")

		router.HandleFunc("/shows/shows", SearchShows).Name("search_shows")
		router.HandleFunc("/shows/popular", PopularShows).Name("popular_shows")
		router.HandleFunc("/shows/{showId}/seasons", ShowSeasons).Name("show_seasons")
		router.HandleFunc("/shows/{showId}/seasons/{season}/episodes", ShowEpisodes).Name("show_season_episodes")
		router.HandleFunc("/shows/{showId}/seasons/{season}/episodes/{episode}/links", ShowEpisodeLinks).Name("show_episode_links")

		router.HandleFunc("/play/{uri}", Play(btService)).Name("play")

		routes = router
	}

	return routes
}

func UrlFor(name string, args ...string) string {
	url, err := routes.Get(name).URLPath(args...)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return PREFIX + url.String()
}
