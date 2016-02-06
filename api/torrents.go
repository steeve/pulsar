package api

import (
	"fmt"
	"errors"
	"strconv"

	"github.com/op/go-logging"
	"github.com/gin-gonic/gin"
	"github.com/scakemyer/libtorrent-go"
	"github.com/scakemyer/quasar/bittorrent"
	"github.com/scakemyer/quasar/xbmc"
)

var torrentsLog = logging.MustGetLogger("torrents")

func ListTorrents(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		btService.Session.GetTorrents()
		torrentsVector := btService.Session.GetTorrents()
		torrentsVectorSize := int(torrentsVector.Size())
		items := make(xbmc.ListItems, 0, torrentsVectorSize)

		torrentsLog.Info("Currently downloading:")
		for i := 0; i < torrentsVectorSize; i++ {
			torrentHandle := torrentsVector.Get(i)
			if torrentHandle.IsValid() == false {
				continue
			}

			torrentName := torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName)).GetName()
			torrentsLog.Info(fmt.Sprintf("- %s", torrentName))

			playUrl := UrlQuery(UrlForXBMC("/play"), "resume", fmt.Sprintf("%d", i))

			item := xbmc.ListItem{
				Label: torrentName,
				Path: playUrl,
				Info: &xbmc.ListItemInfo{
					Title: torrentName,
				},
			}
			item.ContextMenu = [][]string{
				[]string{"LOCALIZE[30230]", fmt.Sprintf("XBMC.PlayMedia(%s)", playUrl)},
				[]string{"LOCALIZE[30231]", fmt.Sprintf("XBMC.PlayMedia(%s)", UrlForXBMC("/torrents/pause/%d", i))},
				[]string{"LOCALIZE[30232]", fmt.Sprintf("XBMC.PlayMedia(%s)", UrlForXBMC("/torrents/delete/%d", i))},
			}
			item.IsPlayable = true
			items = append(items, &item)
		}

		ctx.JSON(200, xbmc.NewView("", items))
	}
}

func PauseTorrent(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		btService.Session.GetTorrents()
		torrentsVector := btService.Session.GetTorrents()
		torrentId := ctx.Params.ByName("torrentId")
		torrentIndex, _ := strconv.Atoi(torrentId)
		torrentHandle := torrentsVector.Get(torrentIndex)
		if torrentHandle.IsValid() == false {
			ctx.Error(errors.New("Invalid torrent handle"))
		}
		torrentInfo := torrentHandle.TorrentFile()

		if torrentInfo != nil && torrentInfo.Swigcptr() != 0 {
			libtorrent.DeleteTorrentInfo(torrentInfo)
		}

		torrentsLog.Info(fmt.Sprintf("Pausing torrent %s", torrentHandle.Status(uint(0)).GetName()))
		torrentHandle.AutoManaged(false)
		torrentHandle.Pause(1)
		ctx.String(200, "")
	}
}

func RemoveTorrent(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		btService.Session.GetTorrents()
		torrentsVector := btService.Session.GetTorrents()
		torrentId := ctx.Params.ByName("torrentId")
		torrentIndex, _ := strconv.Atoi(torrentId)
		torrentHandle := torrentsVector.Get(torrentIndex)
		if torrentHandle.IsValid() == false {
			ctx.Error(errors.New("Invalid torrent handle"))
		}
		torrentInfo := torrentHandle.TorrentFile()

		if torrentInfo != nil && torrentInfo.Swigcptr() != 0 {
			libtorrent.DeleteTorrentInfo(torrentInfo)
		}

		torrentsLog.Info(fmt.Sprintf("Removing torrent %s and deleting files...", torrentHandle.Status(uint(0)).GetName()))
		btService.Session.RemoveTorrent(torrentHandle, int(libtorrent.SessionDeleteFiles))
		ctx.String(200, "")
	}
}
