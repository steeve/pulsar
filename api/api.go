package api

import (
	"encoding/json"
	"net/http"

	"github.com/steeve/pulsar/xbmc"
)

func Index(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(xbmc.NewView("", xbmc.ListItems{
		{Label: "Search Movies", Path: UrlFor("search_movies")},
		{Label: "Popular Movies", Path: UrlFor("popular_movies")},
		{Label: "Movies by Genre", Path: UrlFor("movie_genres")},

		{Label: "Search Shows", Path: UrlFor("search_shows")},
		{Label: "Popular Shows", Path: UrlFor("popular_shows")},
	}))
}
