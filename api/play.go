package api

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/bittorrent"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/util"
	"github.com/scakemyer/quasar/xbmc"
)

func Play(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		uri := ctx.Request.URL.Query().Get("uri")
		index := ctx.Request.URL.Query().Get("index")
		resume := ctx.Request.URL.Query().Get("resume")

		if uri == "" && resume == "" {
			return
		}

		fileIndex := -1
		if index != "" {
			fIndex, err := strconv.Atoi(index)
			if err == nil {
				fileIndex = fIndex
			}
		}

		resumeIndex := -1
		if resume != "" {
			rIndex, err := strconv.Atoi(resume)
			if err == nil && rIndex >= 0 {
				resumeIndex = rIndex
			}
		}

		magnet := ""
		infoHash := ""
		if uri != "" {
			torrent := bittorrent.NewTorrent(uri)
			magnet = torrent.Magnet()
			infoHash = torrent.InfoHash
			boosters := url.Values{
				"tr": bittorrent.DefaultTrackers,
			}
			magnet += "&" + boosters.Encode()
		}

		player := bittorrent.NewBTPlayer(btService, magnet, fileIndex, resumeIndex, infoHash)
		if player.Buffer() != nil {
			return
		}
		hostname := "localhost"
		if localIP, err := util.LocalIP(); err == nil {
			hostname = localIP.String()
		}
		rUrl, _ := url.Parse(fmt.Sprintf("http://%s:%d/files/%s", hostname, config.ListenPort, player.PlayURL()))
		ctx.Redirect(302, rUrl.String())
	}
}

func PasteURL(ctx *gin.Context) {
	magnet := xbmc.Keyboard("", "LOCALIZE[30217]")
	if magnet == "" {
		return
	}
	xbmc.PlayURL(UrlQuery(UrlForXBMC("/play"), "uri", magnet))
}
