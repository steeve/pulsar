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
	"github.com/i96751414/libtorrent-go"
	"github.com/i96751414/pulsar/broadcast"
	"github.com/i96751414/pulsar/config"
	"github.com/i96751414/pulsar/diskusage"
	//"github.com/i96751414/pulsar/ga"
	"github.com/i96751414/pulsar/xbmc"
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
	torrentHandle            libtorrent.Torrent_handle
	torrentInfo              libtorrent.Torrent_info
	biggestFile              libtorrent.File_entry
	lastStatus               libtorrent.Torrent_status
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

	torrentParams := libtorrent.NewAdd_torrent_params()
	defer libtorrent.DeleteAdd_torrent_params(torrentParams)

	torrentParams.SetUrl(btp.uri)

	btp.log.Info("Setting save path to %s\n", btp.bts.config.DownloadPath)
	torrentParams.SetSave_path(btp.bts.config.DownloadPath)

	btp.torrentHandle = btp.bts.Session.Add_torrent(torrentParams)
	go btp.consumeAlerts()

	status := btp.torrentHandle.Status(uint(libtorrent.Torrent_handleQuery_name))

	btp.torrentName = status.GetName()

	if btp.torrentHandle == nil {
		return fmt.Errorf("unable to add torrent with uri %s", btp.uri)
	}

	btp.log.Info("Enabling sequential download")
	btp.torrentHandle.Set_sequential_download(true)

	btp.log.Info("Downloading %s\n", btp.torrentName)

	if status.GetHas_metadata() == true {
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

	btp.torrentInfo = btp.torrentHandle.Torrent_file()

	if btp.diskStatus != nil {
		btp.log.Info("Checking for sufficient space on %s...", btp.bts.config.DownloadPath)
		torrentSize := btp.torrentInfo.Total_size()
		if btp.diskStatus.Free < torrentSize {
			btp.log.Info("Unsufficient free space on %s. Has %d, needs %d.", btp.bts.config.DownloadPath, btp.diskStatus.Free, torrentSize)
			xbmc.Notify("Pulsar", "LOCALIZE[30207]", config.AddonIcon())
			btp.bufferEvents.Broadcast(errors.New("Not enough space on download destination."))
			return
		}
	}

	btp.biggestFile = btp.findBiggestFile()
	btp.log.Info("Biggest file: %s", btp.biggestFile.GetPath())

	btp.log.Info("Setting piece priorities")

	pieceLength := float64(btp.torrentInfo.Piece_length())

	startPiece, endPiece, _ := btp.getFilePiecesAndOffset(btp.biggestFile)

	startLength := float64(endPiece-startPiece) * float64(pieceLength) * startBufferPercent
	if startLength < float64(btp.bts.config.BufferSize) {
		startLength = float64(btp.bts.config.BufferSize)
	}
	startBufferPieces := int(math.Ceil(startLength / pieceLength))

	// Prefer a fixed size, since metadata are very rarely over endPiecesSize=10MB
	// anyway.
	endBufferPieces := int(math.Ceil(float64(endBufferSize) / pieceLength))

	piecesPriorities := libtorrent.NewStd_vector_int()
	defer libtorrent.DeleteStd_vector_int(piecesPriorities)

	btp.bufferPiecesProgressLock.Lock()
	defer btp.bufferPiecesProgressLock.Unlock()

	// Properly set the pieces priority vector
	curPiece := 0
	for _ = 0; curPiece < startPiece; curPiece++ {
		piecesPriorities.Add(0)
	}
	for _ = 0; curPiece < startPiece+startBufferPieces; curPiece++ { // get this part
		piecesPriorities.Add(1)
		btp.bufferPiecesProgress[curPiece] = 0
		btp.torrentHandle.Set_piece_deadline(curPiece, 0, 0)
	}
	for _ = 0; curPiece < endPiece-endBufferPieces; curPiece++ {
		piecesPriorities.Add(1)
	}
	for _ = 0; curPiece <= endPiece; curPiece++ { // get this part
		piecesPriorities.Add(7)
		btp.bufferPiecesProgress[curPiece] = 0
		btp.torrentHandle.Set_piece_deadline(curPiece, 0, 0)
	}
	numPieces := btp.torrentInfo.Num_pieces()
	for _ = 0; curPiece < numPieces; curPiece++ {
		piecesPriorities.Add(0)
	}
	btp.torrentHandle.Prioritize_pieces(piecesPriorities)
}

func (btp *BTPlayer) statusStrings(progress float64, status libtorrent.Torrent_status) (string, string, string) {
	line1 := fmt.Sprintf("%s (%.2f%%)", statusStrings[int(status.GetState())], progress*100)
	if btp.torrentInfo != nil && btp.torrentInfo.Swigcptr() != 0 {
		line1 += " - " + humanize.Bytes(uint64(btp.torrentInfo.Total_size()))
	}
	line2 := fmt.Sprintf("%.0fkb/s S:%d/%d P:%d/%d",
		float64(status.GetDownload_rate())/1024,
		status.GetNum_seeds(),
		status.GetNum_complete(),
		status.GetNum_peers(),
		status.GetNum_incomplete(),
	)
	line3 := status.GetName()
	return line1, line2, line3
}

func (btp *BTPlayer) pieceFromOffset(offset int64) (int, int64) {
	pieceLength := int64(btp.torrentInfo.Piece_length())
	piece := int(offset / pieceLength)
	pieceOffset := offset % pieceLength
	return piece, pieceOffset
}

func (btp *BTPlayer) getFilePiecesAndOffset(fe libtorrent.File_entry) (int, int, int64) {
	startPiece, offset := btp.pieceFromOffset(fe.GetOffset())
	endPiece, _ := btp.pieceFromOffset(fe.GetOffset() + fe.GetSize())
	return startPiece, endPiece, offset
}

func (btp *BTPlayer) findBiggestFile() libtorrent.File_entry {
	var biggestFile libtorrent.File_entry
	maxSize := int64(0)
	numFiles := btp.torrentInfo.Num_files()

	for i := 0; i < numFiles; i++ {
		fe := btp.torrentInfo.File_at(i)
		size := fe.GetSize()
		if size > maxSize {
			maxSize = size
			biggestFile = fe
		}
	}
	return biggestFile
}

func (btp *BTPlayer) onStateChanged(stateAlert libtorrent.State_changed_alert) {
	switch stateAlert.GetState() {
	case libtorrent.Torrent_statusFinished:
		btp.log.Info("Buffer is finished, resetting piece priorities...")
		piecesPriorities := libtorrent.NewStd_vector_int()
		defer libtorrent.DeleteStd_vector_int(piecesPriorities)
		numPieces := btp.torrentInfo.Num_pieces()
		for i := 0; i < numPieces; i++ {
			piecesPriorities.Add(1)
		}
		btp.torrentHandle.Prioritize_pieces(piecesPriorities)
		break
	}
}

func (btp *BTPlayer) Close() {
	close(btp.closing)

	if btp.torrentInfo != nil && btp.torrentInfo.Swigcptr() != 0 {
		libtorrent.DeleteTorrent_info(btp.torrentInfo)
	}

	if btp.deleteAfter {
		btp.log.Info("Removing the torrent and deleting files...")
		btp.bts.Session.Remove_torrent(btp.torrentHandle, int(libtorrent.SessionDelete_files))
	} else {
		btp.log.Info("Removing the torrent without deleting files...")
		btp.bts.Session.Remove_torrent(btp.torrentHandle, 0)
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
			switch alert.Xtype() {
			case libtorrent.Metadata_received_alertAlert_type:
				metadataAlert := libtorrent.SwigcptrMetadata_received_alert(alert.Swigcptr())
				if metadataAlert.GetHandle().Equal(btp.torrentHandle) {
					btp.onMetadataReceived()
				}
				break
			case libtorrent.State_changed_alertAlert_type:
				stateAlert := libtorrent.SwigcptrState_changed_alert(alert.Swigcptr())
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
	queue := libtorrent.NewStd_vector_partial_piece_info()
	defer libtorrent.DeleteStd_vector_partial_piece_info(queue)

	btp.torrentHandle.Get_download_queue(queue)
	for piece, _ := range pieces {
		if btp.torrentHandle.Have_piece(piece) == true {
			pieces[piece] = 1.0
		}
	}
	queueSize := queue.Size()
	for i := 0; i < int(queueSize); i++ {
		ppi := queue.Get(i)
		pieceIndex := ppi.GetPiece_index()
		if _, exists := pieces[pieceIndex]; exists {
			blocks := ppi.Blocks()
			totalBlocks := ppi.GetBlocks_in_piece()
			totalBlockDownloaded := uint(0)
			totalBlockSize := uint(0)
			for j := 0; j < totalBlocks; j++ {
				block := blocks.Getitem(j)
				totalBlockDownloaded += block.GetBytes_progress()
				totalBlockSize += block.GetBlock_size()
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
			status := btp.torrentHandle.Status(uint(libtorrent.Torrent_handleQuery_name))

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
					btp.bufferEvents.Signal()
					return
				}
			}
		}
	}
}

func (btp *BTPlayer) playerLoop() {
	defer btp.Close()

	//start := time.Now()

	btp.log.Info("Buffer loop")

	buffered, bufferDone := btp.bufferEvents.Listen()
	defer close(bufferDone)

	go btp.bufferDialog()

	if err := <-buffered; err != nil {
		return
	}

	//ga.TrackTiming("player", "buffer_time_real", int(time.Now().Sub(start).Seconds()*1000), "")

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
		// 	ga.TrackEvent("player", "waiting_playback", btp.torrentName, -1)
		}
	}

	//ga.TrackTiming("player", "buffer_time_perceived", int(time.Now().Sub(start).Seconds()*1000), "")

	btp.log.Info("Playback loop")
	overlayStatusActive := false
	//playingTicker := time.NewTicker(60 * time.Second)
	//defer playingTicker.Stop()
playbackLoop:
	for {
		if xbmc.PlayerIsPlaying() == false {
			btp.overlayStatus.Close()
			break playbackLoop
		}
		select {
		//case <-playingTicker.C:
		// 	ga.TrackEvent("player", "playing", btp.torrentName, -1)
		case <-oneSecond.C:
			if xbmc.PlayerIsPaused() && config.Get().EnableOverlayStatus == true {
				status := btp.torrentHandle.Status(uint(libtorrent.Torrent_handleQuery_name))
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

	//ga.TrackEvent("player", "stop", btp.torrentName, -1)
	//ga.TrackTiming("player", "watched_time", int(time.Now().Sub(start).Seconds()*1000), "")
}
