package api

import (
	"os"
	"fmt"
	"time"
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

type TorrentMap struct {
	tmdbId  string
	torrent *bittorrent.Torrent
}
var TorrentsMap []*TorrentMap

func AddToTorrentsMap(tmdbId string, torrent *bittorrent.Torrent) {
	inTorrentsMap := false
	for _, torrentMap := range TorrentsMap {
		if tmdbId == torrentMap.tmdbId {
			inTorrentsMap = true
		}
	}
	if inTorrentsMap == false {
		torrentMap := &TorrentMap{
			tmdbId: tmdbId,
			torrent: torrent,
		}
		TorrentsMap = append(TorrentsMap, torrentMap)
	}
}

func InTorrentsMap(tmdbId string) (torrents []*bittorrent.Torrent) {
	for index, torrentMap := range TorrentsMap {
		if tmdbId == torrentMap.tmdbId {
			if xbmc.DialogConfirm("Quasar", "LOCALIZE[30260]") {
				torrents = append(torrents, torrentMap.torrent)
			} else {
				TorrentsMap = append(TorrentsMap[:index], TorrentsMap[index + 1:]...)
			}
		}
	}
	return torrents
}

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

			playUrl := UrlQuery(UrlForXBMC("/play"), "resume", fmt.Sprintf("%d", i))

			status := bittorrent.StatusStrings[int(torrentStatus.GetState())]

			ratio := float64(0)
			allTimeDownload := float64(torrentStatus.GetAllTimeDownload())
			if allTimeDownload > 0 {
				ratio = float64(torrentStatus.GetAllTimeUpload()) / allTimeDownload
			}

			timeRatio := float64(0)
			finished_time := float64(torrentStatus.GetFinishedTime())
			download_time := float64(torrentStatus.GetActiveTime()) - finished_time
			if download_time > 1 {
				timeRatio = finished_time / download_time
			}

			seedingTime := time.Duration(torrentStatus.GetSeedingTime()) * time.Second

			torrentAction := []string{"LOCALIZE[30231]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/torrents/pause/%d", i))}
			sessionAction := []string{"LOCALIZE[30233]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/torrents/pause"))}

			if torrentStatus.GetPaused() {
				status = "Paused"
				torrentAction = []string{"LOCALIZE[30235]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/torrents/resume/%d", i))}
			}
			if btService.Session.IsPaused() {
				status = "Paused"
				sessionAction = []string{"LOCALIZE[30234]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/torrents/resume"))}
			}
			torrentsLog.Infof("- %.2f%% - %s - %.2f:1 / %.2f:1 (%s) - %s", progress, status, ratio, timeRatio, seedingTime.String(), torrentName)

			item := xbmc.ListItem{
				Label: fmt.Sprintf("%.2f%% - %s - %.2f:1 / %.2f:1 (%s) - %s", progress, status, ratio, timeRatio, seedingTime.String(), torrentName),
				Path: playUrl,
				Info: &xbmc.ListItemInfo{
					Title: torrentName,
				},
			}
			item.ContextMenu = [][]string{
				[]string{"LOCALIZE[30230]", fmt.Sprintf("XBMC.PlayMedia(%s)", playUrl)},
				torrentAction,
				[]string{"LOCALIZE[30232]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/torrents/delete/%d", i))},
				sessionAction,
			}
			item.IsPlayable = true
			items = append(items, &item)
		}

		ctx.JSON(200, xbmc.NewView("", items))
	}
}

func PauseSession(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		btService.Session.Pause()
		xbmc.Refresh()
		ctx.String(200, "")
	}
}

func ResumeSession(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		btService.Session.Resume()
		xbmc.Refresh()
		ctx.String(200, "")
	}
}

func ResumeTorrent(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		torrentsVector := btService.Session.GetTorrents()
		torrentId := ctx.Params.ByName("torrentId")
		torrentIndex, _ := strconv.Atoi(torrentId)
		torrentHandle := torrentsVector.Get(torrentIndex)

		if torrentHandle == nil {
			ctx.Error(errors.New(fmt.Sprintf("Unable to resume torrent with index %d", torrentIndex)))
		}

		status := torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName))

		torrentName := status.GetName()
		torrentsLog.Infof("Resuming %s", torrentName)

		torrentHandle.AutoManaged(true)

		xbmc.Refresh()
		ctx.String(200, "")
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

		xbmc.Refresh()
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
		fastResumeFile := filepath.Join(config.Get().TorrentsPath, fmt.Sprintf("%s.fastresume", infoHash))
		if _, err := os.Stat(fastResumeFile); err == nil {
			torrentsLog.Infof("Deleting fast resume data at %s", fastResumeFile)
			defer os.Remove(fastResumeFile)
		}

		if config.Get().KeepFilesAfterStop == false {
			torrentsLog.Info("Removing the torrent and deleting files...")
			btService.Session.RemoveTorrent(torrentHandle, int(libtorrent.SessionDeleteFiles))
		} else {
			torrentsLog.Info("Removing the torrent without deleting files...")
			btService.Session.RemoveTorrent(torrentHandle, 0)
		}

		xbmc.Refresh()
		ctx.String(200, "")
	}
}
