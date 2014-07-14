package api

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/steeve/pulsar/bittorrent"
)

func Play(btService *bittorrent.BTService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		uri := vars["uri"]
		log.Println(uri)
		player := bittorrent.NewBTPlayer(btService, uri)
		if player.Buffer() != nil {
			return
		}
		rUrl, _ := url.Parse(fmt.Sprintf("http://localhost:8000/files/%s", player.PlayURL()))
		log.Println(rUrl)
		http.Redirect(w, r, rUrl.String(), http.StatusFound)
	}
}
