package api

import (
	"fmt"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/pulsar/bittorrent"
	"github.com/scakemyer/pulsar/config"
	"github.com/scakemyer/pulsar/providers"
	"github.com/scakemyer/pulsar/util"
	"github.com/scakemyer/pulsar/xbmc"
)

func Play(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		uri := ctx.Request.URL.Query().Get("uri")
		if uri == "" {
			return
		}
		torrent := bittorrent.NewTorrent(uri)
		magnet := torrent.Magnet()
		boosters := url.Values{
			"tr": providers.DefaultTrackers,
		}
		magnet += "&" + boosters.Encode()
		player := bittorrent.NewBTPlayer(btService, magnet, config.Get().KeepFilesAfterStop == false)
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
