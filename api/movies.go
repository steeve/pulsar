package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/providers"
	"github.com/steeve/pulsar/providers/cli"
	"github.com/steeve/pulsar/tmdb"
	"github.com/steeve/pulsar/xbmc"
)

func renderMovies(movies tmdb.Movies, w http.ResponseWriter, r *http.Request) {
	items := make(xbmc.ListItems, 0, len(movies))
	for _, movie := range movies {
		item := movie.ToListItem()
		item.Path = UrlFor("movie_play", "imdbId", movie.IMDBId)
		item.IsPlayable = true
		items = append(items, item)
	}

	json.NewEncoder(w).Encode(xbmc.NewView("movies", items))
}

func PopularMovies(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	genre := vars["genre"]
	if genre == "0" {
		genre = ""
	}
	renderMovies(tmdb.PopularMoviesComplete(genre), w, r)
}

func SearchMovies(w http.ResponseWriter, r *http.Request) {
	query := xbmc.Keyboard("", "Search Movies")
	if query != "" {
		renderMovies(tmdb.SearchMovies(query), w, r)
	}
}

func MovieGenres(w http.ResponseWriter, r *http.Request) {
	genres := tmdb.GetMovieGenres()
	items := make(xbmc.ListItems, 0, len(genres))
	for _, genre := range genres {
		items = append(items, &xbmc.ListItem{
			Label: genre.Name,
			Path:  UrlFor("popular_movies_by_genre", "genre", strconv.Itoa(genre.Id)),
		})
	}

	json.NewEncoder(w).Encode(xbmc.NewView("", items))
}

func movieLinks(imdbId string) []*bittorrent.Torrent {
	log.Println("Searching links for IMDB:", imdbId)

	movie := tmdb.GetMovieFromIMDB(imdbId)

	log.Printf("Resolved %s to %s\n", imdbId, movie.Title)

	searchers := []providers.MovieSearcher{
		cli.NewCLISearcher([]string{"python", "kat.py"}),
		cli.NewCLISearcher([]string{"python", "tpb.py"}),
		cli.NewCLISearcher([]string{"python", "yts.py"}),
	}

	return providers.SearchMovie(searchers, movie)
}

func MovieLinks(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	torrents := movieLinks(vars["imdbId"])
	choices := make([]string, 0, len(torrents))
	for _, torrent := range torrents {
		info := make([]string, 0, 4)
		if torrent.RipType > 0 {
			info = append(info, bittorrent.Rips[torrent.RipType])
		}
		if torrent.Resolution > 0 {
			info = append(info, bittorrent.Resolutions[torrent.Resolution])
		}
		if torrent.VideoCodec > 0 {
			info = append(info, bittorrent.Codecs[torrent.VideoCodec])
		}
		if torrent.AudioCodec > 0 {
			info = append(info, bittorrent.Codecs[torrent.AudioCodec])
		}

		label := fmt.Sprintf("S:%d P:%d - %s - %s",
			torrent.Seeds,
			torrent.Peers,
			strings.Join(info, " "),
			torrent.Name,
		)
		choices = append(choices, label)
	}

	choice := xbmc.ListDialog("Choose your stream", choices...)
	if choice >= 0 {
		rUrl := UrlFor("play", torrents[choice].URI)
		// rUrl := fmt.Sprintf("plugin://plugin.video.pulsar/play/%s", url.QueryEscape(torrents[choice].URI))
		http.Redirect(w, r, rUrl, http.StatusFound)
	}
}

func MoviePlay(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	torrents := movieLinks(vars["imdbId"])
	// rUrl := fmt.Sprintf("plugin://plugin.video.xbmctorrent/play/%s", url.QueryEscape(torrents[0].URI))
	rUrl := UrlFor("play", "uri", torrents[0].URI)
	http.Redirect(w, r, rUrl, http.StatusFound)
}
