package api

import (
	"fmt"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/i96751414/pulsar/bittorrent"
	"github.com/i96751414/pulsar/config"
	"github.com/i96751414/pulsar/providers"
	"github.com/i96751414/pulsar/util"
	"github.com/i96751414/pulsar/xbmc"
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
	magnet := xbmc.Keyboard("", "Paste Magnet or URL")
	if magnet == "" {
		return
	}
	xbmc.PlayURL(UrlQuery(UrlForXBMC("/play"), "uri", magnet))
}
