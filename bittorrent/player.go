package bittorrent

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/op/go-logging"
	"github.com/steeve/libtorrent-go"
	"github.com/steeve/pulsar/broadcast"
	"github.com/steeve/pulsar/diskusage"
	"github.com/steeve/pulsar/ga"
	"github.com/steeve/pulsar/xbmc"
)

const (
	startPiecesBuffer = 0.01
	endPiecesSize     = 10 * 1024 * 1024 // 10m
	playbackMaxWait   = 20 * time.Second
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
	bts            *BTService
	uri            string
	torrentHandle  libtorrent.Torrent_handle
	torrentInfo    libtorrent.Torrent_info
	biggestFile    libtorrent.File_entry
	log            *logging.Logger
	dialogProgress *xbmc.DialogProgress
	torrentName    string
	deleteAfter    bool
	diskStatus     *diskusage.DiskStatus
	closing        chan interface{}
	didBuffer      *broadcast.Broadcaster
}

func NewBTPlayer(bts *BTService, uri string, deleteAfter bool) *BTPlayer {
	btp := &BTPlayer{
		bts:         bts,
		uri:         uri,
		log:         logging.MustGetLogger("btplayer"),
		deleteAfter: deleteAfter,
		closing:     make(chan interface{}),
		didBuffer:   broadcast.NewBroadcaster(),
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

	buffered, done := btp.didBuffer.Listen()
	defer close(done)

	btp.dialogProgress = xbmc.NewDialogProgress("Pulsar", "", "", "")
	defer btp.dialogProgress.Close()

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
	btp.torrentName = btp.torrentHandle.Status(uint(0)).GetName()
	go ga.TrackEvent("player", "metadata_received", btp.torrentName, -1)

	btp.torrentInfo = btp.torrentHandle.Torrent_file()

	if btp.diskStatus != nil {
		btp.log.Info("Checking for sufficient space on %s...", btp.bts.config.DownloadPath)
		torrentSize := btp.torrentInfo.Total_size()
		if btp.diskStatus.Free < torrentSize {
			btp.log.Info("Unsufficient free space on %s. Has %d, needs %d.", btp.bts.config.DownloadPath, btp.diskStatus.Free, torrentSize)
			xbmc.Notify("Pulsar", "Not enough space available on the download path.")
			return
		}
	}

	btp.biggestFile = btp.findBiggestFile()
	btp.log.Info("Biggest file: %s", btp.biggestFile.GetPath())

	btp.log.Info("Setting piece priorities")
	startPiece, endPiece, _ := btp.getFilePiecesAndOffset(btp.biggestFile)
	startBufferPieces := int(math.Ceil(float64(endPiece-startPiece) * startPiecesBuffer))

	// Prefer a fixed size, since metadata are very rarely over endPiecesSize=10MB
	// anyway.
	endBufferPieces := int(math.Ceil(float64(endPiecesSize) / float64(btp.torrentInfo.Piece_length())))

	piecesPriorities := libtorrent.NewStd_vector_int()
	defer libtorrent.DeleteStd_vector_int(piecesPriorities)

	// Properly set the pieces priority vector
	curPiece := 0
	for _ = 0; curPiece < startPiece; curPiece++ {
		piecesPriorities.Add(0)
	}
	for _ = 0; curPiece < startPiece+startBufferPieces; curPiece++ { // get this part
		piecesPriorities.Add(1)
	}
	for _ = 0; curPiece < endPiece-endBufferPieces; curPiece++ {
		piecesPriorities.Add(0)
	}
	for _ = 0; curPiece <= endPiece; curPiece++ { // get this part
		piecesPriorities.Add(1)
	}
	numPieces := btp.torrentInfo.Num_pieces()
	for _ = 0; curPiece < numPieces; curPiece++ {
		piecesPriorities.Add(0)
	}
	btp.torrentHandle.Prioritize_pieces(piecesPriorities)
}

func (btp *BTPlayer) statusStrings(status libtorrent.Torrent_status) (string, string, string) {
	line1 := fmt.Sprintf("%s (%.2f%%)", statusStrings[int(status.GetState())], status.GetProgress()*100)
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

		btp.didBuffer.Signal()
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

func (btp *BTPlayer) playerLoop() {
	defer btp.Close()

	start := time.Now()

	secondTicker := time.Tick(1 * time.Second)

	btp.log.Info("Buffer loop")
	buffered, done := btp.didBuffer.Listen()
	defer close(done)
bufferLoop:
	for {
		select {
		case <-buffered:
			btp.log.Info("Buffer complete")
			break bufferLoop
		case <-secondTicker:
			ga.TrackEvent("player", "buffering", btp.torrentName, -1)
			if btp.dialogProgress.IsCanceled() {
				btp.log.Info("User cancelled the buffering")
				go ga.TrackEvent("player", "buffer_canceled", btp.torrentName, -1)
				btp.didBuffer.Write(fmt.Errorf("user canceled the buffering"))
				return
			}
			status := btp.torrentHandle.Status(uint(libtorrent.Torrent_handleQuery_name))
			line1, line2, line3 := btp.statusStrings(status)
			btp.dialogProgress.Update(int(status.GetProgress()*100.0), line1, line2, line3)
		}
	}
	ga.TrackTiming("player", "buffer_time_real", int(time.Now().Sub(start).Seconds()*1000), "")

	btp.log.Info("Waiting for playback...")
	playbackTimeout := time.After(playbackMaxWait)
playbackWaitLoop:
	for {
		if xbmc.PlayerIsPlaying() {
			break playbackWaitLoop
		}
		select {
		case <-playbackTimeout:
			btp.log.Info("Playback was unable to start after %d seconds. Aborting...", playbackMaxWait)
			btp.didBuffer.Write(fmt.Errorf("Playback was unable to start before timeout."))
			return
		case <-secondTicker:
			ga.TrackEvent("player", "waiting_playback", btp.torrentName, -1)
		}
	}

	ga.TrackTiming("player", "buffer_time_perceived", int(time.Now().Sub(start).Seconds()*1000), "")

	btp.log.Info("Playback loop")
	playingTicker := time.Tick(60 * time.Second)
playbackLoop:
	for {
		if xbmc.PlayerIsPlaying() == false {
			break playbackLoop
		}
		select {
		case <-playingTicker:
			ga.TrackEvent("player", "playing", btp.torrentName, -1)
		case <-secondTicker:
		}
	}

	ga.TrackEvent("player", "stop", btp.torrentName, -1)
	ga.TrackTiming("player", "watched_time", int(time.Now().Sub(start).Seconds()*1000), "")
}
