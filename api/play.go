package api

import (
	"fmt"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/config"
	"github.com/steeve/pulsar/util"
)

func Play(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		uri := ctx.Request.URL.Query().Get("uri")
		if uri == "" {
			return
		}
		torrent := bittorrent.NewTorrent(uri)
		player := bittorrent.NewBTPlayer(btService, torrent.Magnet(), config.Get().KeepFilesAfterStop == false)
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
