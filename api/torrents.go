package api

import (
	"os"
	"fmt"
	"errors"
	"strconv"
	"encoding/hex"
	"path/filepath"

	"github.com/op/go-logging"
	"github.com/gin-gonic/gin"
	"github.com/scakemyer/libtorrent-go"
	"github.com/scakemyer/quasar/bittorrent"
	"github.com/scakemyer/quasar/config"
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

			torrentStatus := torrentHandle.Status()
			progress := float64(torrentStatus.GetProgress()) * 100
			torrentName := torrentStatus.GetName()
			torrentsLog.Info("- " + torrentName)

			playUrl := UrlQuery(UrlForHTTP("/play"), "resume", fmt.Sprintf("%d", i))

			status := bittorrent.StatusStrings[int(torrentStatus.GetState())]
			if torrentStatus.GetPaused() {
				status = "Paused"
			}

			item := xbmc.ListItem{
				Label: fmt.Sprintf("%.2f%% - %s - %s", progress, status, torrentName),
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

		// Delete fast resume data
		torrentStatus := torrentHandle.Status(uint(libtorrent.TorrentHandleQuerySavePath) | uint(libtorrent.TorrentHandleQueryName))
		shaHash := torrentStatus.GetInfoHash().ToString()
		infoHash := hex.EncodeToString([]byte(shaHash))
		fastResumeFile := filepath.Join(config.Get().DownloadPath, fmt.Sprintf("%s.fastresume", infoHash))
		if _, err := os.Stat(fastResumeFile); err == nil {
			torrentsLog.Info("Deleting fast resume data at %s", fastResumeFile)
			defer os.Remove(fastResumeFile)
		}

		if config.Get().KeepFilesAfterStop == false {
			torrentsLog.Info("Removing the torrent and deleting files...")
			btService.Session.RemoveTorrent(torrentHandle, int(libtorrent.SessionDeleteFiles))
		} else {
			torrentsLog.Info("Removing the torrent without deleting files...")
			btService.Session.RemoveTorrent(torrentHandle, 0)
		}

		ctx.Redirect(302, UrlForXBMC("/torrents/"))
	}
}
