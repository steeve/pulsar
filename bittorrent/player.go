package bittorrent

import (
	"os"
	"fmt"
	"math"
	"sync"
	"time"
	"errors"
	"strings"
	"io/ioutil"
	"encoding/hex"
	"path/filepath"

	"github.com/op/go-logging"
	"github.com/dustin/go-humanize"
	"github.com/scakemyer/libtorrent-go"
	"github.com/scakemyer/quasar/broadcast"
	"github.com/scakemyer/quasar/diskusage"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/trakt"
	"github.com/scakemyer/quasar/xbmc"
)

const (
	startBufferPercent = 0.005
	endBufferSize      = 10 * 1024 * 1024 // 10m
	playbackMaxWait    = 20 * time.Second
	minCandidateSize   = 100 * 1024 * 1024
)

type BTPlayer struct {
	bts                      *BTService
	log                      *logging.Logger
	dialogProgress           *xbmc.DialogProgress
	overlayStatus            *xbmc.OverlayStatus
	uri                      string
	fastResumeFile           string
	contentType              string
	fileIndex                int
	resumeIndex              int
	tmdbId                   int
	runtime                  int
	scrobble                 bool
	deleteAfter              bool
	askToKeep                bool
	backgroundHandling       bool
	overlayStatusEnabled     bool
	torrentHandle            libtorrent.TorrentHandle
	torrentInfo              libtorrent.TorrentInfo
	chosenFile               int
	lastStatus               libtorrent.TorrentStatus
	bufferPiecesProgress     map[int]float64
	bufferPiecesProgressLock sync.RWMutex
	torrentName              string
	notEnoughSpace           bool
	diskStatus               *diskusage.DiskStatus
	bufferEvents             *broadcast.Broadcaster
	closing                  chan interface{}
}

type BTPlayerParams struct {
	URI          string
	FileIndex    int
	ResumeIndex  int
	ContentType  string
	TMDBId       int
	Runtime      int
}

func NewBTPlayer(bts *BTService, params BTPlayerParams) *BTPlayer {
	btp := &BTPlayer{
		log:                  logging.MustGetLogger("btplayer"),
		bts:                  bts,
		uri:                  params.URI,
		fileIndex:            params.FileIndex,
		resumeIndex:          params.ResumeIndex,
		overlayStatusEnabled: config.Get().EnableOverlayStatus == true,
		backgroundHandling:   config.Get().BackgroundHandling == true,
		deleteAfter:          config.Get().KeepFilesAfterStop == false,
		askToKeep:            config.Get().KeepFilesAsk == true,
		scrobble:             config.Get().Scrobble == true && params.TMDBId > 0 && config.Get().TraktToken != "",
		contentType:          params.ContentType,
		tmdbId:               params.TMDBId,
		runtime:              params.Runtime * 60,
		fastResumeFile:       "",
		notEnoughSpace:       false,
		closing:              make(chan interface{}),
		bufferEvents:         broadcast.NewBroadcaster(),
		bufferPiecesProgress: map[int]float64{},
	}
	return btp
}

func (btp *BTPlayer) addTorrent() error {
	btp.log.Infof("Adding torrent from %s", btp.uri)

	if btp.bts.config.DownloadPath == "." {
		xbmc.Notify("Quasar", "LOCALIZE[30113]", config.AddonIcon())
		return fmt.Errorf("Download path empty")
	}

	if status, err := diskusage.DiskUsage(btp.bts.config.DownloadPath); err != nil {
		btp.bts.log.Warningf("Unable to retrieve the free space for %s, continuing anyway...", btp.bts.config.DownloadPath)
	} else {
		btp.diskStatus = status
	}

	torrentParams := libtorrent.NewAddTorrentParams()
	defer libtorrent.DeleteAddTorrentParams(torrentParams)

	var infoHash string

	if strings.HasPrefix(btp.uri, "magnet") || strings.HasPrefix(btp.uri, "http") {
		torrentParams.SetUrl(btp.uri)

		torrent := NewTorrent(btp.uri)
		torrent.Magnet()
		infoHash = torrent.InfoHash
	} else {
		info := libtorrent.NewTorrentInfo(btp.uri) // FIXME crashes on invalid paths
		torrentParams.SetTorrentInfo(info)

		shaHash := info.InfoHash().ToString()
		infoHash = hex.EncodeToString([]byte(shaHash))
	}

	btp.log.Infof("Setting save path to %s", btp.bts.config.DownloadPath)
	torrentParams.SetSavePath(btp.bts.config.DownloadPath)

	btp.log.Infof("Checking for fast resume data in %s.fastresume", infoHash)
	fastResumeFile := filepath.Join(btp.bts.config.TorrentsPath, fmt.Sprintf("%s.fastresume", infoHash))
	btp.fastResumeFile = fastResumeFile
	if _, err := os.Stat(fastResumeFile); err == nil {
		btp.log.Info("Found fast resume data...")
		fastResumeData, err := ioutil.ReadFile(fastResumeFile)
		if err != nil {
			return err
		}
		fastResumeVector := libtorrent.NewStdVectorChar()
		defer libtorrent.DeleteStdVectorChar(fastResumeVector)
		for _, c := range fastResumeData {
			fastResumeVector.Add(c)
		}
		torrentParams.SetResumeData(fastResumeVector)
	}

	btp.torrentHandle = btp.bts.Session.GetHandle().AddTorrent(torrentParams) // FIXME crashes on invalid magnet
	go btp.consumeAlerts()

	if btp.torrentHandle == nil {
		return fmt.Errorf("Unable to add torrent with URI %s", btp.uri)
	}

	btp.log.Info("Enabling sequential download")
	btp.torrentHandle.SetSequentialDownload(true)

	status := btp.torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName))

	btp.torrentName = status.GetName()
	btp.log.Infof("Downloading %s", btp.torrentName)

	if status.GetHasMetadata() == true {
		btp.onMetadataReceived()
	}

	return nil
}

func (btp *BTPlayer) resumeTorrent(torrentIndex int) error {
	torrentsVector := btp.bts.Session.GetHandle().GetTorrents()
	btp.torrentHandle = torrentsVector.Get(torrentIndex)
	go btp.consumeAlerts()

	if btp.torrentHandle == nil {
		return fmt.Errorf("Unable to resume torrent with index %d", torrentIndex)
	}

	status := btp.torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName))

	btp.torrentName = status.GetName()
	btp.log.Infof("Resuming %s", btp.torrentName)

	if status.GetHasMetadata() == true {
		btp.onMetadataReceived()
	}

	btp.torrentHandle.AutoManaged(true)

	return nil
}

func (btp *BTPlayer) Buffer() error {
	if btp.resumeIndex >= 0 {
		if err := btp.resumeTorrent(btp.resumeIndex); err != nil {
			return err
		}
	} else {
		if err := btp.addTorrent(); err != nil {
			return err
		}
	}

	buffered, done := btp.bufferEvents.Listen()
	defer close(done)

	btp.dialogProgress = xbmc.NewDialogProgress("Quasar", "", "", "")
	defer btp.dialogProgress.Close()

	btp.overlayStatus = xbmc.NewOverlayStatus()

	go btp.playerLoop()

	if err := <-buffered; err != nil {
		return err.(error)
	}
	return nil
}

func (btp *BTPlayer) PlayURL() string {
	return strings.Join(strings.Split(btp.torrentInfo.Files().FilePath(btp.chosenFile), string(os.PathSeparator)), "/")
}

func (btp *BTPlayer) CheckAvailableSpace() bool {
	if btp.diskStatus != nil {
		if btp.torrentInfo == nil || btp.torrentInfo.Swigcptr() == 0 {
			btp.log.Warning("Missing torrent info to check available space.")
			return true
		}

		status := btp.torrentHandle.Status(uint(libtorrent.TorrentHandleQueryAccurateDownloadCounters))
		sizeLeft := btp.torrentInfo.TotalSize() - status.GetTotalDone()
		availableSpace := btp.diskStatus.Free

		btp.log.Infof("Checking for sufficient space on %s...", btp.bts.config.DownloadPath)
		btp.log.Infof("Total size of download: %d", btp.torrentInfo.TotalSize())
		btp.log.Infof("All time download: %d", status.GetAllTimeDownload())
		btp.log.Infof("Size total done: %d", status.GetTotalDone())
		btp.log.Infof("Size left to download: %d", sizeLeft)
		btp.log.Infof("Available space: %d", availableSpace)

		if availableSpace < sizeLeft {
			btp.log.Errorf("Unsufficient free space on %s. Has %d, needs %d.", btp.bts.config.DownloadPath, btp.diskStatus.Free, sizeLeft)
			xbmc.Notify("Quasar", "LOCALIZE[30207]", config.AddonIcon())
			btp.bufferEvents.Broadcast(errors.New("Not enough space on download destination."))
			btp.notEnoughSpace = true
			return false
		}
	}
	return true
}

func (btp *BTPlayer) onMetadataReceived() {
	btp.log.Info("Metadata received.")

	btp.torrentName = btp.torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName)).GetName()

	btp.torrentInfo = btp.torrentHandle.TorrentFile()

	// Reset fastResumeFile
	shaHash := btp.torrentInfo.InfoHash().ToString()
	infoHash := hex.EncodeToString([]byte(shaHash))
	btp.fastResumeFile = filepath.Join(btp.bts.config.TorrentsPath, fmt.Sprintf("%s.fastresume", infoHash))

	var err error
	btp.chosenFile, err = btp.chooseFile()
	if err != nil {
		btp.bufferEvents.Broadcast(errors.New("User cancelled."))
		return
	}
	btp.log.Infof("Chosen file: %s", btp.torrentInfo.Files().FilePath(btp.chosenFile))

	btp.log.Info("Setting piece priorities")

	pieceLength := float64(btp.torrentInfo.PieceLength())

	startPiece, endPiece, _ := btp.getFilePiecesAndOffset(btp.chosenFile)

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
		piecesPriorities.Add(0)
	}
	for _ = 0; curPiece < startPiece+startBufferPieces; curPiece++ { // get this part
		piecesPriorities.Add(7)
		btp.bufferPiecesProgress[curPiece] = 0
		btp.torrentHandle.SetPieceDeadline(curPiece, 0, 0)
	}
	for _ = 0; curPiece < endPiece-endBufferPieces; curPiece++ {
		piecesPriorities.Add(1)
	}
	for _ = 0; curPiece <= endPiece; curPiece++ { // get this part
		piecesPriorities.Add(7)
		btp.bufferPiecesProgress[curPiece] = 0
		btp.torrentHandle.SetPieceDeadline(curPiece, 0, 0)
	}
	numPieces := btp.torrentInfo.NumPieces()
	for _ = 0; curPiece < numPieces; curPiece++ {
		piecesPriorities.Add(0)
	}
	btp.torrentHandle.PrioritizePieces(piecesPriorities)
}

func (btp *BTPlayer) statusStrings(progress float64, status libtorrent.TorrentStatus) (string, string, string) {
	line1 := fmt.Sprintf("%s (%.2f%%)", StatusStrings[int(status.GetState())], progress*100)
	if btp.torrentInfo != nil && btp.torrentInfo.Swigcptr() != 0 {
		line1 += " - " + humanize.Bytes(uint64(btp.torrentInfo.TotalSize()))
	}
	seeders := status.GetNumSeeds()
	line2 := fmt.Sprintf("D:%.0fkB/s U:%.0fkB/s S:%d/%d P:%d/%d",
		float64(status.GetDownloadRate())/1024,
		float64(status.GetUploadRate())/1024,
		seeders,
		status.GetNumComplete(),
		status.GetNumPeers() - seeders,
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

func (btp *BTPlayer) getFilePiecesAndOffset(fe int) (int, int, int64) {
	files := btp.torrentInfo.Files()
	startPiece, offset := btp.pieceFromOffset(files.FileOffset(fe))
	endPiece, _ := btp.pieceFromOffset(files.FileOffset(fe) + files.FileSize(fe))
	return startPiece, endPiece, offset
}

func (btp *BTPlayer) chooseFile() (int, error) {
	var biggestFile int
	maxSize := int64(0)
	numFiles := btp.torrentInfo.NumFiles()
	files := btp.torrentInfo.Files()
	var candidateFiles []int

	for i := 0; i < numFiles; i++ {
		size := files.FileSize(i)
		if size > maxSize {
			maxSize = size
			biggestFile = i
		}
		if size > minCandidateSize {
			candidateFiles = append(candidateFiles, i)
		}
	}

	if len(candidateFiles) > 1 {
		btp.log.Info(fmt.Sprintf("There are %d candidate files", len(candidateFiles)))
		if btp.fileIndex >= 0 && btp.fileIndex < len(candidateFiles) {
			return candidateFiles[btp.fileIndex], nil
		}
		choices := make([]string, 0, len(candidateFiles))
		for _, index := range candidateFiles {
			fileName := filepath.Base(files.FilePath(index))
			choices = append(choices, fileName)
		}
		choice := xbmc.ListDialog("LOCALIZE[30223]", choices...)
		if choice >= 0 {
			return candidateFiles[choice], nil
		} else {
			return biggestFile, fmt.Errorf("User cancelled")
		}
	}

	return biggestFile, nil
}

func (btp *BTPlayer) onStateChanged(stateAlert libtorrent.StateChangedAlert) {
	switch stateAlert.GetState() {
	case libtorrent.TorrentStatusFinished:
		btp.log.Info("Buffer is finished, resetting piece priorities...")
		piecesPriorities := libtorrent.NewStdVectorInt()
		defer libtorrent.DeleteStdVectorInt(piecesPriorities)
		numPieces := btp.torrentInfo.NumPieces()
		for i := 0; i < numPieces; i++ {
			piecesPriorities.Add(1)
		}
		btp.torrentHandle.PrioritizePieces(piecesPriorities)
		break
	case libtorrent.TorrentStatusDownloading:
		btp.CheckAvailableSpace()
		break
	}
}

func (btp *BTPlayer) Close() {
	close(btp.closing)

	askedToKeep := false
	if btp.askToKeep == true {
		if xbmc.DialogConfirm("Quasar", "LOCALIZE[30267]") {
			askedToKeep = true
		}
	} else {
		askedToKeep = true
	}

	if btp.backgroundHandling == false || askedToKeep == false || btp.notEnoughSpace {
		// Delete fast resume data
		if _, err := os.Stat(btp.fastResumeFile); err == nil {
			btp.log.Infof("Deleting fast resume data at %s", btp.fastResumeFile)
			defer os.Remove(btp.fastResumeFile)
		}

		if btp.deleteAfter || askedToKeep == false {
			btp.log.Info("Removing the torrent and deleting files...")
			btp.bts.Session.GetHandle().RemoveTorrent(btp.torrentHandle, int(libtorrent.SessionHandleDeleteFiles))
		} else {
			btp.log.Info("Removing the torrent without deleting files...")
			btp.bts.Session.GetHandle().RemoveTorrent(btp.torrentHandle, 0)
		}
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
			switch alert.Type {
			case libtorrent.MetadataReceivedAlertAlertType:
				metadataAlert := libtorrent.SwigcptrMetadataReceivedAlert(alert.Pointer)
				if metadataAlert.GetHandle().Equal(btp.torrentHandle) {
					btp.onMetadataReceived()
				}
				break
			case libtorrent.StateChangedAlertAlertType:
				stateAlert := libtorrent.SwigcptrStateChangedAlert(alert.Pointer)
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
			if btp.dialogProgress.IsCanceled() || btp.notEnoughSpace {
				btp.log.Info("User cancelled the buffering")
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
		settings := btp.bts.packSettings
		if enable == true {
			if btp.bts.config.MaxDownloadRate > 0 {
				btp.log.Infof("Buffer filled, rate limiting download to %dkB/s", btp.bts.config.MaxDownloadRate/1024)
				settings.SetInt(libtorrent.SettingByName("download_rate_limit"), btp.bts.config.MaxDownloadRate)
			}
			if btp.bts.config.MaxUploadRate > 0 {
				// If we have an upload rate, use the nicer bittyrant choker
				btp.log.Infof("Buffer filled, rate limiting upload to %dkB/s", btp.bts.config.MaxUploadRate/1024)
				settings.SetInt(libtorrent.SettingByName("upload_rate_limit"), btp.bts.config.MaxUploadRate)
			}
		} else {
			btp.log.Info("Resetting rate limiting")
			settings.SetInt(libtorrent.SettingByName("download_rate_limit"), 0)
			settings.SetInt(libtorrent.SettingByName("upload_rate_limit"), 0)
		}
		btp.bts.Session.GetHandle().ApplySettings(settings)
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
			btp.log.Warningf("Playback was unable to start after %d seconds. Aborting...", playbackMaxWait / time.Second)
			btp.bufferEvents.Broadcast(errors.New("Playback was unable to start before timeout."))
		 	return
		case <-oneSecond.C:
		}
	}

	btp.log.Info("Playback loop")
	overlayStatusActive := false
	playing := true
	btp.log.Infof("Got runtime: %d", btp.runtime)
	if btp.scrobble {
		trakt.Scrobble("start", btp.contentType, btp.tmdbId, btp.runtime)
	}

playbackLoop:
	for {
		if xbmc.PlayerIsPlaying() == false {
			break playbackLoop
		} else if btp.scrobble {
			trakt.Scrobble("update", btp.contentType, btp.tmdbId, btp.runtime)
		}
		select {
		case <-oneSecond.C:
			if xbmc.PlayerIsPaused() {
				if btp.scrobble && playing == true {
					playing = false
					trakt.Scrobble("pause", btp.contentType, btp.tmdbId, btp.runtime)
				}
				if btp.overlayStatusEnabled == true {
					status := btp.torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName))
					progress := float64(status.GetProgress())
					line1, line2, line3 := btp.statusStrings(progress, status)
					btp.overlayStatus.Update(int(progress), line1, line2, line3)
					if overlayStatusActive == false {
						btp.overlayStatus.Show()
						overlayStatusActive = true
					}
				}
			} else {
				if btp.scrobble && playing == false {
					playing = true
					trakt.Scrobble("start", btp.contentType, btp.tmdbId, btp.runtime)
				}
				if overlayStatusActive == true {
					btp.overlayStatus.Hide()
					overlayStatusActive = false
				}
			}
		}
	}

	btp.overlayStatus.Close()
	btp.setRateLimiting(false)

	if btp.scrobble {
		trakt.Scrobble("stop", btp.contentType, btp.tmdbId, btp.runtime)
	}
}
