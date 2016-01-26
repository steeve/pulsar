package bittorrent

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/op/go-logging"
	"github.com/scakemyer/libtorrent-go"
	"github.com/scakemyer/pulsar/broadcast"
	"github.com/scakemyer/pulsar/config"
	"github.com/scakemyer/pulsar/diskusage"
	"github.com/scakemyer/pulsar/xbmc"
)

const (
	startBufferPercent = 0.005
	endBufferSize      = 10 * 1024 * 1024 // 10m
	playbackMaxWait    = 20 * time.Second
)

var statusStrings = []string{
	"Queued",
	"Checking",
	"Finding",
	"Buffering",
	"Finished",
	"Seeding",
	"Allocating",
	"Stalled",
}

type BTPlayer struct {
	bts                      *BTService
	uri                      string
	torrentHandle            libtorrent.TorrentHandle
	torrentInfo              libtorrent.TorrentInfo
	biggestFile              libtorrent.FileEntry
	lastStatus               libtorrent.TorrentStatus
	log                      *logging.Logger
	bufferPiecesProgress     map[int]float64
	bufferPiecesProgressLock sync.RWMutex
	dialogProgress           *xbmc.DialogProgress
	overlayStatus            *xbmc.OverlayStatus
	torrentName              string
	deleteAfter              bool
	diskStatus               *diskusage.DiskStatus
	closing                  chan interface{}
	bufferEvents             *broadcast.Broadcaster
}

func NewBTPlayer(bts *BTService, uri string, deleteAfter bool) *BTPlayer {
	btp := &BTPlayer{
		bts:                  bts,
		uri:                  uri,
		log:                  logging.MustGetLogger("btplayer"),
		deleteAfter:          deleteAfter,
		closing:              make(chan interface{}),
		bufferEvents:         broadcast.NewBroadcaster(),
		bufferPiecesProgress: map[int]float64{},
	}
	return btp
}

func (btp *BTPlayer) addTorrent() error {
	btp.log.Info("Adding torrent")

	if status, err := diskusage.DiskUsage(btp.bts.config.DownloadPath); err != nil {
		btp.bts.log.Info("Unable to retrieve the free space for %s, continuing anyway...", btp.bts.config.DownloadPath)
	} else {
		btp.diskStatus = status
	}

	torrentParams := libtorrent.NewAddTorrentParams()
	defer libtorrent.DeleteAddTorrentParams(torrentParams)

	torrentParams.SetUrl(btp.uri)

	btp.log.Info("Setting save path to %s\n", btp.bts.config.DownloadPath)
	torrentParams.SetSavePath(btp.bts.config.DownloadPath)

	btp.torrentHandle = btp.bts.Session.AddTorrent(torrentParams)
	go btp.consumeAlerts()

	status := btp.torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName))

	btp.torrentName = status.GetName()

	if btp.torrentHandle == nil {
		return fmt.Errorf("unable to add torrent with uri %s", btp.uri)
	}

	btp.log.Info("Enabling sequential download")
	btp.torrentHandle.SetSequentialDownload(true)

	btp.log.Info("Downloading %s\n", btp.torrentName)

	if status.GetHasMetadata() == true {
		btp.onMetadataReceived()
	}

	return nil
}

func (btp *BTPlayer) Buffer() error {
	if err := btp.addTorrent(); err != nil {
		return err
	}

	buffered, done := btp.bufferEvents.Listen()
	defer close(done)

	btp.dialogProgress = xbmc.NewDialogProgress("Pulsar", "", "", "")
	defer btp.dialogProgress.Close()

	btp.overlayStatus = xbmc.NewOverlayStatus()

	go btp.playerLoop()

	if err := <-buffered; err != nil {
		return err.(error)
	}
	return nil
}

func (btp *BTPlayer) PlayURL() string {
	return strings.Join(strings.Split(btp.biggestFile.GetPath(), string(os.PathSeparator)), "/")
}

func (btp *BTPlayer) onMetadataReceived() {
	btp.log.Info("Metadata received.")

	btp.torrentHandle.Pause()
	defer btp.torrentHandle.Resume()

	btp.torrentName = btp.torrentHandle.Status(uint(0)).GetName()
	//go ga.TrackEvent("player", "metadata_received", btp.torrentName, -1)

	btp.torrentInfo = btp.torrentHandle.TorrentFile()

	if btp.diskStatus != nil {
		btp.log.Info("Checking for sufficient space on %s...", btp.bts.config.DownloadPath)
		torrentSize := btp.torrentInfo.TotalSize()
		if btp.diskStatus.Free < torrentSize {
			btp.log.Info("Unsufficient free space on %s. Has %d, needs %d.", btp.bts.config.DownloadPath, btp.diskStatus.Free, torrentSize)
			xbmc.Notify("Pulsar", "LOCALIZE[30207]", config.AddonIcon())
			// btp.bufferEvents.Broadcast(errors.New("Not enough space on download destination."))
			// return
		}
	}

	btp.biggestFile = btp.findBiggestFile()
	btp.log.Info("Biggest file: %s", btp.biggestFile.GetPath())

	btp.log.Info("Setting piece priorities")

	pieceLength := float64(btp.torrentInfo.PieceLength())

	startPiece, endPiece, _ := btp.getFilePiecesAndOffset(btp.biggestFile)

	startLength := float64(endPiece-startPiece) * float64(pieceLength) * startBufferPercent
	if startLength < float64(btp.bts.config.BufferSize) {
		startLength = float64(btp.bts.config.BufferSize)
	}
	startBufferPieces := int(math.Ceil(startLength / pieceLength))

	// Prefer a fixed size, since metadata are very rarely over endPiecesSize=10MB
	// anyway.
	endBufferPieces := int(math.Ceil(float64(endBufferSize) / pieceLength))

	piecesPriorities := libtorrent.NewStdVectorInt()
	defer libtorrent.DeleteStdVectorInt(piecesPriorities)

	btp.bufferPiecesProgressLock.Lock()
	defer btp.bufferPiecesProgressLock.Unlock()

	// Properly set the pieces priority vector
	curPiece := 0
	for _ = 0; curPiece < startPiece; curPiece++ {
		piecesPriorities.PushBack(0)
	}
	for _ = 0; curPiece < startPiece+startBufferPieces; curPiece++ { // get this part
		piecesPriorities.PushBack(1)
		btp.bufferPiecesProgress[curPiece] = 0
		btp.torrentHandle.SetPieceDeadline(curPiece, 0, 0)
	}
	for _ = 0; curPiece < endPiece-endBufferPieces; curPiece++ {
		piecesPriorities.PushBack(1)
	}
	for _ = 0; curPiece <= endPiece; curPiece++ { // get this part
		piecesPriorities.PushBack(7)
		btp.bufferPiecesProgress[curPiece] = 0
		btp.torrentHandle.SetPieceDeadline(curPiece, 0, 0)
	}
	numPieces := btp.torrentInfo.NumPieces()
	for _ = 0; curPiece < numPieces; curPiece++ {
		piecesPriorities.PushBack(0)
	}
	btp.torrentHandle.PrioritizePieces(piecesPriorities)
}

func (btp *BTPlayer) statusStrings(progress float64, status libtorrent.TorrentStatus) (string, string, string) {
	line1 := fmt.Sprintf("%s (%.2f%%)", statusStrings[int(status.GetState())], progress*100)
	if btp.torrentInfo != nil && btp.torrentInfo.Swigcptr() != 0 {
		line1 += " - " + humanize.Bytes(uint64(btp.torrentInfo.TotalSize()))
	}
	line2 := fmt.Sprintf("D:%.0fkb/s U:%.0fkb/s S:%d/%d P:%d/%d",
		float64(status.GetDownloadRate())/1024,
		float64(status.GetUploadRate())/1024,
		status.GetNumSeeds(),
		status.GetNumComplete(),
		status.GetNumPeers(),
		status.GetNumIncomplete(),
	)
	line3 := status.GetName()
	return line1, line2, line3
}

func (btp *BTPlayer) pieceFromOffset(offset int64) (int, int64) {
	pieceLength := int64(btp.torrentInfo.PieceLength())
	piece := int(offset / pieceLength)
	pieceOffset := offset % pieceLength
	return piece, pieceOffset
}

func (btp *BTPlayer) getFilePiecesAndOffset(fe libtorrent.FileEntry) (int, int, int64) {
	startPiece, offset := btp.pieceFromOffset(fe.GetOffset())
	endPiece, _ := btp.pieceFromOffset(fe.GetOffset() + fe.GetSize())
	return startPiece, endPiece, offset
}

func (btp *BTPlayer) findBiggestFile() libtorrent.FileEntry {
	var biggestFile libtorrent.FileEntry
	maxSize := int64(0)
	numFiles := btp.torrentInfo.NumFiles()

	for i := 0; i < numFiles; i++ {
		fe := btp.torrentInfo.FileAt(i)
		size := fe.GetSize()
		if size > maxSize {
			maxSize = size
			biggestFile = fe
		}
	}
	return biggestFile
}

func (btp *BTPlayer) onStateChanged(stateAlert libtorrent.StateChangedAlert) {
	switch stateAlert.GetState() {
	case libtorrent.TorrentStatusFinished:
		btp.log.Info("Buffer is finished, resetting piece priorities...")
		piecesPriorities := libtorrent.NewStdVectorInt()
		defer libtorrent.DeleteStdVectorInt(piecesPriorities)
		numPieces := btp.torrentInfo.NumPieces()
		for i := 0; i < numPieces; i++ {
			piecesPriorities.PushBack(1)
		}
		btp.torrentHandle.PrioritizePieces(piecesPriorities)
		break
	}
}

func (btp *BTPlayer) Close() {
	close(btp.closing)

	if btp.torrentInfo != nil && btp.torrentInfo.Swigcptr() != 0 {
		libtorrent.DeleteTorrentInfo(btp.torrentInfo)
	}

	if btp.deleteAfter {
		btp.log.Info("Removing the torrent and deleting files...")
		btp.bts.Session.RemoveTorrent(btp.torrentHandle, int(libtorrent.SessionDeleteFiles))
	} else {
		btp.log.Info("Removing the torrent without deleting files...")
		btp.bts.Session.RemoveTorrent(btp.torrentHandle, 0)
	}
}

func (btp *BTPlayer) consumeAlerts() {
	alerts, alertsDone := btp.bts.Alerts()
	defer close(alertsDone)

	for {
		select {
		case alert, ok := <-alerts:
			if !ok { // was the alerts channel closed?
				return
			}
			switch alert.Type() {
			case libtorrent.MetadataReceivedAlertAlertType:
				metadataAlert := libtorrent.SwigcptrMetadataReceivedAlert(alert.Swigcptr())
				if metadataAlert.GetHandle().Equal(btp.torrentHandle) {
					btp.onMetadataReceived()
				}
				break
			case libtorrent.StateChangedAlertAlertType:
				stateAlert := libtorrent.SwigcptrStateChangedAlert(alert.Swigcptr())
				if stateAlert.GetHandle().Equal(btp.torrentHandle) {
					btp.onStateChanged(stateAlert)
				}
				break
			}
		case <-btp.closing:
			return
		}
	}
}

func (btp *BTPlayer) piecesProgress(pieces map[int]float64) {
	queue := libtorrent.NewStdVectorPartialPieceInfo()
	defer libtorrent.DeleteStdVectorPartialPieceInfo(queue)

	btp.torrentHandle.GetDownloadQueue(queue)
	for piece, _ := range pieces {
		if btp.torrentHandle.HavePiece(piece) == true {
			pieces[piece] = 1.0
		}
	}
	queueSize := queue.Size()
	for i := 0; i < int(queueSize); i++ {
		ppi := queue.Get(i)
		pieceIndex := ppi.GetPieceIndex()
		if _, exists := pieces[pieceIndex]; exists {
			blocks := ppi.Blocks()
			totalBlocks := ppi.GetBlocksInPiece()
			totalBlockDownloaded := uint(0)
			totalBlockSize := uint(0)
			for j := 0; j < totalBlocks; j++ {
				block := blocks.Getitem(j)
				totalBlockDownloaded += block.GetBytesProgress()
				totalBlockSize += block.GetBlockSize()
			}
			pieces[pieceIndex] = float64(totalBlockDownloaded) / float64(totalBlockSize)
		}
	}
}

func (btp *BTPlayer) bufferDialog() {
	halfSecond := time.NewTicker(500 * time.Millisecond)
	defer halfSecond.Stop()
	oneSecond := time.NewTicker(1 * time.Second)
	defer oneSecond.Stop()

	for {
		select {
		case <-halfSecond.C:
			if btp.dialogProgress.IsCanceled() {
				btp.log.Info("User cancelled the buffering")
				//go ga.TrackEvent("player", "buffer_canceled", btp.torrentName, -1)
				btp.bufferEvents.Broadcast(errors.New("user canceled the buffering"))
				return
			}
		case <-oneSecond.C:
			status := btp.torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName))

			// Handle "Checking" state for resumed downloads
			if int(status.GetState()) == 1 {
				progress := float64(status.GetProgress())
				line1, line2, line3 := btp.statusStrings(progress, status)
				btp.dialogProgress.Update(int(progress*100.0), line1, line2, line3)
			} else {
				bufferProgress := float64(0)
				btp.bufferPiecesProgressLock.Lock()
				if len(btp.bufferPiecesProgress) > 0 {
					totalProgress := float64(0)
					btp.piecesProgress(btp.bufferPiecesProgress)
					for _, v := range btp.bufferPiecesProgress {
						totalProgress += v
					}
					bufferProgress = totalProgress / float64(len(btp.bufferPiecesProgress))
				}
				btp.bufferPiecesProgressLock.Unlock()
				line1, line2, line3 := btp.statusStrings(bufferProgress, status)
				btp.dialogProgress.Update(int(bufferProgress*100.0), line1, line2, line3)
				if bufferProgress >= 1 {
					btp.setRateLimiting(true)
					btp.bufferEvents.Signal()
					return
				}
			}
		}
	}
}

func (btp *BTPlayer) setRateLimiting(enable bool) {
	if btp.bts.config.LimitAfterBuffering == true {
		settings := btp.bts.Session.Settings()
		if enable == true {
			if btp.bts.config.MaxDownloadRate > 0 {
				btp.log.Info("Buffer filled, rate limiting download to %dkb/s", btp.bts.config.MaxDownloadRate/1024)
				settings.SetDownloadRateLimit(btp.bts.config.MaxDownloadRate)
			}
			if btp.bts.config.MaxUploadRate > 0 {
				// If we have an upload rate, use the nicer bittyrant choker
				btp.log.Info("Buffer filled, rate limiting upload to %dkb/s", btp.bts.config.MaxUploadRate/1024)
				settings.SetUploadRateLimit(btp.bts.config.MaxUploadRate)
			}
		} else {
			btp.log.Info("Resetting rate limiting")
			settings.SetDownloadRateLimit(0)
			settings.SetUploadRateLimit(0)
		}
		btp.bts.Session.SetSettings(settings)
	}
}

func (btp *BTPlayer) playerLoop() {
	defer btp.Close()

	btp.log.Info("Buffer loop")

	buffered, bufferDone := btp.bufferEvents.Listen()
	defer close(bufferDone)

	go btp.bufferDialog()

	if err := <-buffered; err != nil {
		return
	}

	btp.log.Info("Waiting for playback...")
	oneSecond := time.NewTicker(1 * time.Second)
	defer oneSecond.Stop()
	playbackTimeout := time.After(playbackMaxWait)

playbackWaitLoop:
	for {
		if xbmc.PlayerIsPlaying() {
			break playbackWaitLoop
		}
		select {
		case <-playbackTimeout:
			btp.log.Info("Playback was unable to start after %d seconds. Aborting...", playbackMaxWait/time.Second)
			btp.bufferEvents.Broadcast(errors.New("Playback was unable to start before timeout."))
		 	return
		case <-oneSecond.C:
		}
	}

	btp.log.Info("Playback loop")
	overlayStatusActive := false

playbackLoop:
	for {
		if xbmc.PlayerIsPlaying() == false {
			break playbackLoop
		}
		select {
		case <-oneSecond.C:
			if xbmc.PlayerIsPaused() && config.Get().EnableOverlayStatus == true {
				status := btp.torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName))
				progress := float64(status.GetProgress())
				line1, line2, line3 := btp.statusStrings(progress, status)
				btp.overlayStatus.Update(int(progress), line1, line2, line3)
				if overlayStatusActive == false {
					btp.overlayStatus.Show()
					overlayStatusActive = true
				}
			} else {
				if overlayStatusActive == true {
					btp.overlayStatus.Hide()
					overlayStatusActive = false
				}
			}
		}
	}
	btp.overlayStatus.Close()
	btp.setRateLimiting(false)
}
