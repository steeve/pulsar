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
	"github.com/steeve/pulsar/ga"
	"github.com/steeve/pulsar/xbmc"
)

const (
	startPiecesBuffer = 0.01
	endPiecesSize     = 10 * 1024 * 1024 // 10m
	playbackMaxWait   = 20
)

var statusStrings = []string{
	"Queued",
	"Checking...",
	"Finding torrent...", //"Downloading metadata...",
	"Buffering...",       // "Downloading",
	"Finished",
	"Seeding...",
	"Allocating",
	"Allocating file & Checking resume",
}

type BTPlayer struct {
	bts            *BTService
	uri            string
	torrentHandle  libtorrent.Torrent_handle
	torrentInfo    libtorrent.Torrent_info
	biggestFile    libtorrent.File_entry
	log            *logging.Logger
	waitBuffer     chan error
	dialogProgress *xbmc.DialogProgress
	isBuffering    bool
	torrentName    string
	deleteAfter    bool
	closing        chan bool
}

func NewBTPlayer(bts *BTService, uri string, deleteAfter bool) *BTPlayer {
	btp := &BTPlayer{
		bts:         bts,
		uri:         uri,
		log:         logging.MustGetLogger("btplayer"),
		waitBuffer:  make(chan error, 10),
		deleteAfter: deleteAfter,
		closing:     make(chan bool),
	}
	return btp
}

func (btp *BTPlayer) addTorrent() error {
	btp.log.Info("Adding torrent")
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
	btp.isBuffering = true

	if err := btp.addTorrent(); err != nil {
		return err
	}

	btp.dialogProgress = xbmc.NewDialogProgress("Pulsar", "", "", "")
	defer btp.dialogProgress.Close()

	go btp.playerLoop()

	return <-btp.waitBuffer
}

func (btp *BTPlayer) PlayURL() string {
	return strings.Join(strings.Split(btp.biggestFile.GetPath(), string(os.PathSeparator)), "/")
}

func (btp *BTPlayer) onMetadataReceived() {
	btp.log.Info("Metadata received.")
	btp.torrentName = btp.torrentHandle.Status(uint(0)).GetName()
	go ga.TrackEvent("player", "metadata_received", btp.torrentName, -1)

	btp.torrentInfo = btp.torrentHandle.Torrent_file()
	biggestFileIdx := btp.findBiggestFile()
	btp.biggestFile = btp.torrentInfo.File_at(biggestFileIdx)
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

func (btp *BTPlayer) findBiggestFile() int {
	maxSize := int64(0)
	biggestFile := 0
	numFiles := btp.torrentInfo.Num_files()
	for i := 0; i < numFiles; i++ {
		fe := btp.torrentInfo.File_at(i)
		size := fe.GetSize()
		if size > maxSize {
			maxSize = size
			biggestFile = i
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
		if btp.isBuffering {
			btp.isBuffering = false
			btp.waitBuffer <- nil
		}
		break
	}
}

func (btp *BTPlayer) Close() {
	btp.log.Info("Removing the torrent...")
	btp.closing <- true

	if btp.deleteAfter {
		btp.bts.Session.Remove_torrent(btp.torrentHandle, int(libtorrent.SessionDelete_files))
	} else {
		btp.bts.Session.Remove_torrent(btp.torrentHandle, 0)
	}
	if btp.torrentInfo != nil && btp.torrentInfo.Swigcptr() != 0 {
		libtorrent.DeleteTorrent_info(btp.torrentInfo)
	}
}

func (btp *BTPlayer) consumeAlerts() {
	alerts := btp.bts.BindAlerts()
	defer btp.bts.UnbindAlerts(alerts)
	for {
		select {
		case alert, ok := <-alerts:
			if !ok {
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

	btp.log.Info("Buffer loop")
	for btp.isBuffering == true {
		ga.TrackEvent("player", "buffering", btp.torrentName, -1)
		if btp.dialogProgress.IsCanceled() {
			btp.log.Info("User cancelled the buffering")
			go ga.TrackEvent("player", "buffer_canceled", btp.torrentName, -1)
			btp.waitBuffer <- fmt.Errorf("user canceled the buffering")
			return
		}
		status := btp.torrentHandle.Status(uint(libtorrent.Torrent_handleQuery_name))
		line1, line2, line3 := btp.statusStrings(status)
		btp.dialogProgress.Update(int(status.GetProgress()*100.0), line1, line2, line3)
		time.Sleep(1000 * time.Millisecond)
	}

	ga.TrackTiming("player", "buffer_time_real", int(time.Now().Sub(start).Seconds()*1000), "")

	btp.log.Info("Waiting for playback...")
	playbackWaited := 0
	for xbmc.PlayerIsPlaying() == false {
		if playbackWaited >= playbackMaxWait {
			btp.log.Info("Playback was unable to start after %d seconds. Aborting...", playbackMaxWait)
			return
		}
		ga.TrackEvent("player", "waiting_playback", btp.torrentName, -1)
		time.Sleep(1000 * time.Millisecond)
		playbackWaited++
	}

	ga.TrackTiming("player", "buffer_time_perceived", int(time.Now().Sub(start).Seconds()*1000), "")

	btp.log.Info("Playback loop")
	for i := 0; xbmc.PlayerIsPlaying(); i++ {
		if i%60 == 0 { // send keep alive every 60 seconds
			ga.TrackEvent("player", "playing", btp.torrentName, -1)
		}
		time.Sleep(1000 * time.Millisecond)
	}

	ga.TrackEvent("player", "stop", btp.torrentName, -1)
	ga.TrackTiming("player", "watched_time", int(time.Now().Sub(start).Seconds()*1000), "")
}
