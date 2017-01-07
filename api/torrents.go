package api

import (
	"os"
	"fmt"
	"time"
	"errors"
	"strings"
	"strconv"
	"io/ioutil"
	"encoding/hex"
	"path/filepath"

	"github.com/op/go-logging"
	"github.com/gin-gonic/gin"
	"github.com/dustin/go-humanize"
	"github.com/scakemyer/libtorrent-go"
	"github.com/scakemyer/quasar/bittorrent"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/xbmc"
)

var torrentsLog = logging.MustGetLogger("torrents")

type TorrentsWeb struct {
	Name         string  `json:"name"`
	Size         string  `json:"size"`
	Status       string  `json:"status"`
	Progress     float64 `json:"progress"`
	Ratio        float64 `json:"ratio"`
	TimeRatio    float64 `json:"time_ratio"`
	SeedingTime  string  `json:"seeding_time"`
	DownloadRate float64 `json:"download_rate"`
	UploadRate   float64 `json:"upload_rate"`
	Seeders      int     `json:"seeders"`
	SeedersTotal int     `json:"seeders_total"`
	Peers        int     `json:"peers"`
	PeersTotal   int     `json:"peers_total"`
}

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
		btService.Session.GetHandle().GetTorrents()
		torrentsVector := btService.Session.GetHandle().GetTorrents()
		torrentsVectorSize := int(torrentsVector.Size())
		items := make(xbmc.ListItems, 0, torrentsVectorSize)

		torrentsLog.Info("Currently downloading:")
		for i := 0; i < torrentsVectorSize; i++ {
			torrentHandle := torrentsVector.Get(i)
			if torrentHandle.IsValid() == false {
				continue
			}

			torrentStatus := torrentHandle.Status()

			torrentName := torrentStatus.GetName()
			progress := float64(torrentStatus.GetProgress()) * 100
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
			if btService.Session.GetHandle().IsPaused() {
				status = "Paused"
				sessionAction = []string{"LOCALIZE[30234]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/torrents/resume"))}
			}
			torrentsLog.Infof("- %.2f%% - %s - %.2f:1 / %.2f:1 (%s) - %s", progress, status, ratio, timeRatio, seedingTime.String(), torrentName)

			playUrl := UrlQuery(UrlForXBMC("/play"), "resume", fmt.Sprintf("%d", i))
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

func ListTorrentsWeb(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		btService.Session.GetHandle().GetTorrents()
		torrentsVector := btService.Session.GetHandle().GetTorrents()
		torrentsVectorSize := int(torrentsVector.Size())
		torrents := make([]*TorrentsWeb, 0, torrentsVectorSize)

		for i := 0; i < torrentsVectorSize; i++ {
			torrentHandle := torrentsVector.Get(i)
			if torrentHandle.IsValid() == false {
				continue
			}

			torrentStatus := torrentHandle.Status()

			torrentName := torrentStatus.GetName()
			progress := float64(torrentStatus.GetProgress()) * 100

			status := bittorrent.StatusStrings[int(torrentStatus.GetState())]
			if torrentStatus.GetPaused() {
				status = "Paused"
			}
			if btService.Session.GetHandle().IsPaused() {
				status = "Paused"
			}

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

			torrentInfo := torrentHandle.TorrentFile()
			size := ""
			if torrentInfo != nil && torrentInfo.Swigcptr() != 0 {
				size = humanize.Bytes(uint64(torrentInfo.TotalSize()))
			}
			downloadRate := float64(torrentStatus.GetDownloadRate()) / 1024
			uploadRate := float64(torrentStatus.GetUploadRate()) / 1024
			seeders := torrentStatus.GetNumSeeds()
			seedersTotal := torrentStatus.GetNumComplete()
			peers := torrentStatus.GetNumPeers() - seeders
			peersTotal := torrentStatus.GetNumIncomplete()

			torrent := TorrentsWeb{
				Name: torrentName,
				Size: size,
				Status: status,
				Progress: progress,
				Ratio: ratio,
				TimeRatio: timeRatio,
				SeedingTime: seedingTime.String(),
				DownloadRate: downloadRate,
				UploadRate: uploadRate,
				Seeders: seeders,
				SeedersTotal: seedersTotal,
				Peers: peers,
				PeersTotal: peersTotal,
			}
			torrents = append(torrents, &torrent)
		}

		ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.JSON(200, torrents)
	}
}

func PauseSession(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		btService.Session.GetHandle().Pause()
		xbmc.Refresh()
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.String(200, "")
	}
}

func ResumeSession(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		btService.Session.GetHandle().Resume()
		xbmc.Refresh()
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.String(200, "")
	}
}

func AddTorrent(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		uri := ctx.Request.URL.Query().Get("uri")
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")

		if uri == "" {
			ctx.String(200, "Missing torrent URI")
		}
		torrentsLog.Infof("Adding torrent from %s", uri)

		if config.Get().DownloadPath == "." {
			xbmc.Notify("Quasar", "LOCALIZE[30113]", config.AddonIcon())
			ctx.String(200, "Download path empty")
			return
		}

		torrentParams := libtorrent.NewAddTorrentParams()
		defer libtorrent.DeleteAddTorrentParams(torrentParams)

		var infoHash string

		if strings.HasPrefix(uri, "magnet") || strings.HasPrefix(uri, "http") {
			torrentParams.SetUrl(uri)

			torrent := bittorrent.NewTorrent(uri)
			torrent.Magnet()
			infoHash = torrent.InfoHash
		} else {
			info := libtorrent.NewTorrentInfo(uri) // FIXME crashes on invalid paths
			torrentParams.SetTorrentInfo(info)

			shaHash := info.InfoHash().ToString()
			infoHash = hex.EncodeToString([]byte(shaHash))
		}

		torrentsLog.Infof("Setting save path to %s", config.Get().DownloadPath)
		torrentParams.SetSavePath(config.Get().DownloadPath)

		torrentsLog.Infof("Checking for fast resume data in %s.fastresume", infoHash)
		fastResumeFile := filepath.Join(config.Get().TorrentsPath, fmt.Sprintf("%s.fastresume", infoHash))
		if _, err := os.Stat(fastResumeFile); err == nil {
			torrentsLog.Info("Found fast resume data...")
			fastResumeData, err := ioutil.ReadFile(fastResumeFile)
			if err != nil {
				torrentsLog.Error(err.Error())
				ctx.String(200, err.Error())
				return
			}
			fastResumeVector := libtorrent.NewStdVectorChar()
			defer libtorrent.DeleteStdVectorChar(fastResumeVector)
			for _, c := range fastResumeData {
				fastResumeVector.Add(c)
			}
			torrentParams.SetResumeData(fastResumeVector)
		}

		torrentHandle := btService.Session.GetHandle().AddTorrent(torrentParams) // FIXME crashes on invalid magnet

		if torrentHandle == nil {
			ctx.String(200, fmt.Sprintf("Unable to add torrent with URI %s", uri))
			return
		}

		torrentStatus := torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName))
		torrentsLog.Infof("Downloading %s", torrentStatus.GetName())

		xbmc.Refresh()
		ctx.String(200, "")
	}
}

func ResumeTorrent(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		torrentsVector := btService.Session.GetHandle().GetTorrents()
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
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.String(200, "")
	}
}

func PauseTorrent(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		btService.Session.GetHandle().GetTorrents()
		torrentsVector := btService.Session.GetHandle().GetTorrents()
		torrentId := ctx.Params.ByName("torrentId")
		torrentIndex, _ := strconv.Atoi(torrentId)
		torrentHandle := torrentsVector.Get(torrentIndex)
		if torrentHandle.IsValid() == false {
			ctx.Error(errors.New("Invalid torrent handle"))
		}

		torrentsLog.Infof("Pausing torrent %s", torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName)).GetName())
		torrentHandle.AutoManaged(false)
		torrentHandle.Pause(1)

		xbmc.Refresh()
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.String(200, "")
	}
}

func RemoveTorrent(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		btService.Session.GetHandle().GetTorrents()
		torrentsVector := btService.Session.GetHandle().GetTorrents()
		torrentId := ctx.Params.ByName("torrentId")
		torrentIndex, _ := strconv.Atoi(torrentId)
		torrentHandle := torrentsVector.Get(torrentIndex)
		if torrentHandle.IsValid() == false {
			ctx.Error(errors.New("Invalid torrent handle"))
		}

		torrentStatus := torrentHandle.Status(uint(libtorrent.TorrentHandleQuerySavePath) | uint(libtorrent.TorrentHandleQueryName))
		shaHash := torrentStatus.GetInfoHash().ToString()
		infoHash := hex.EncodeToString([]byte(shaHash))

		// Delete torrent file
		torrentFile := filepath.Join(config.Get().TorrentsPath, fmt.Sprintf("%s.torrent", infoHash))
		if _, err := os.Stat(torrentFile); err == nil {
			torrentsLog.Infof("Deleting torrent file at %s", torrentFile)
			defer os.Remove(torrentFile)
		}

		// Delete fast resume data
		fastResumeFile := filepath.Join(config.Get().TorrentsPath, fmt.Sprintf("%s.fastresume", infoHash))
		if _, err := os.Stat(fastResumeFile); err == nil {
			torrentsLog.Infof("Deleting fast resume data at %s", fastResumeFile)
			defer os.Remove(fastResumeFile)
		}

		askedToDelete := false
		if config.Get().KeepFilesAsk == true {
			if xbmc.DialogConfirm("Quasar", "LOCALIZE[30269]") {
				askedToDelete = true
			}
		}

		if config.Get().KeepFilesAfterStop == false || askedToDelete == true {
			torrentsLog.Info("Removing the torrent and deleting files...")
			btService.Session.GetHandle().RemoveTorrent(torrentHandle, int(libtorrent.SessionHandleDeleteFiles))
		} else {
			torrentsLog.Info("Removing the torrent without deleting files...")
			btService.Session.GetHandle().RemoveTorrent(torrentHandle, 0)
		}

		xbmc.Refresh()
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.String(200, "")
	}
}
