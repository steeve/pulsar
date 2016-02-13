package bittorrent

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/op/go-logging"
	"github.com/scakemyer/libtorrent-go"
	"github.com/scakemyer/quasar/broadcast"
)

const (
	piecesRefreshDuration = 500 * time.Millisecond
)

type TorrentFS struct {
	http.Dir
	service *BTService
	log     *logging.Logger
}

type TorrentFile struct {
	*os.File
	tfs               *TorrentFS
	torrentHandle     libtorrent.TorrentHandle
	torrentInfo       libtorrent.TorrentInfo
	fileEntry         libtorrent.FileEntry
	fileEntryIdx      int
	pieceLength       int
	fileOffset        int64
	fileSize          int64
	piecesMx          sync.RWMutex
	pieces            Bitfield
	piecesLastUpdated time.Time
	lastStatus        libtorrent.TorrentStatus
	removed           *broadcast.Broadcaster
}

func NewTorrentFS(service *BTService, path string) *TorrentFS {
	return &TorrentFS{
		service: service,
		log:     logging.MustGetLogger("torrentfs"),
		Dir:     http.Dir(path),
	}
}

func (tfs *TorrentFS) Open(name string) (http.File, error) {
	file, err := os.Open(filepath.Join(string(tfs.Dir), name))
	if err != nil {
		return nil, err
	}
	// make sure we don't open a file that's locked, as it can happen
	// on BSD systems (darwin included)
	if err := unlockFile(file); err != nil {
		tfs.log.Errorf("Unable to unlock file because: %s", err)
	}

	tfs.log.Infof("Opening %s", name)
	// NB: this does NOT return a pointer to vector, no need to free!
	torrentsVector := tfs.service.Session.GetTorrents()
	torrentsVectorSize := int(torrentsVector.Size())
	for i := 0; i < torrentsVectorSize; i++ {
		torrentHandle := torrentsVector.Get(i)
		if torrentHandle.IsValid() == false {
			continue
		}
		torrentInfo := torrentHandle.TorrentFile()
		numFiles := torrentInfo.NumFiles()
		for j := 0; j < numFiles; j++ {
			fe := torrentInfo.FileAt(j)
			if name[1:] == fe.GetPath() {
				// tfs.log.Infof("%s belongs to torrent %s", name, torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName)).GetName())
				return NewTorrentFile(file, tfs, torrentHandle, torrentInfo, fe, j)
			}
		}
		defer libtorrent.DeleteTorrentInfo(torrentInfo)
	}
	return file, err
}

func NewTorrentFile(file *os.File, tfs *TorrentFS, torrentHandle libtorrent.TorrentHandle, torrentInfo libtorrent.TorrentInfo, fileEntry libtorrent.FileEntry, fileEntryIdx int) (*TorrentFile, error) {
	tf := &TorrentFile{
		File:          file,
		tfs:           tfs,
		torrentHandle: torrentHandle,
		torrentInfo:   torrentInfo,
		fileEntry:     fileEntry,
		fileEntryIdx:  fileEntryIdx,
		pieceLength:   torrentInfo.PieceLength(),
		fileOffset:    fileEntry.GetOffset(),
		fileSize:      fileEntry.GetSize(),
		removed:       broadcast.NewBroadcaster(),
	}
	go tf.consumeAlerts()

	return tf, nil
}

func (tf *TorrentFile) consumeAlerts() {
	alerts, done := tf.tfs.service.Alerts()
	defer close(done)
	for alert := range alerts {
		switch alert.Type() {
		case libtorrent.TorrentRemovedAlertAlertType:
			removedAlert := libtorrent.SwigcptrTorrentAlert(alert.Swigcptr())
			if removedAlert.GetHandle().Equal(tf.torrentHandle) {
				tf.removed.Signal()
				return
			}
		}
	}
}

func (tf *TorrentFile) updatePieces() error {
	tf.piecesMx.Lock()
	defer tf.piecesMx.Unlock()

	if time.Now().After(tf.piecesLastUpdated.Add(piecesRefreshDuration)) {
		// need to keep a reference to the status or else the pieces bitfield
		// is at risk of being collected
		tf.lastStatus = tf.torrentHandle.Status(uint(libtorrent.TorrentHandleQueryPieces))
		if tf.lastStatus.GetState() > libtorrent.TorrentStatusSeeding {
			return errors.New("Torrent file has invalid state.")
		}
		piecesBits := tf.lastStatus.GetPieces()
		piecesBitsSize := piecesBits.Size()
		piecesSliceSize := piecesBitsSize / 8
		if piecesBitsSize%8 > 0 {
			// Add +1 to round up the bitfield
			piecesSliceSize += 1
		}
		data := (*[100000000]byte)(unsafe.Pointer(piecesBits.Bytes()))[:piecesSliceSize]
		tf.pieces = Bitfield(data)
		tf.piecesLastUpdated = time.Now()
	}
	return nil
}

func (tf *TorrentFile) hasPiece(idx int) bool {
	if err := tf.updatePieces(); err != nil {
		return false
	}
	tf.piecesMx.RLock()
	defer tf.piecesMx.RUnlock()
	return tf.pieces.GetBit(idx)
}

func (tf *TorrentFile) Close() error {
	tf.tfs.log.Info("Closing file...")
	tf.removed.Signal()
	libtorrent.DeleteTorrentInfo(tf.torrentInfo)
	return tf.File.Close()
}

func (tf *TorrentFile) Read(data []byte) (int, error) {
	currentOffset, err := tf.File.Seek(0, os.SEEK_CUR)
	if err != nil {
		return 0, err
	}
	// tf.tfs.log.Infof("About to read from file at %d for %d\n", currentOffset, len(data))
	piece, _ := tf.pieceFromOffset(currentOffset + int64(len(data)))
	if err := tf.waitForPiece(piece); err != nil {
		return 0, err
	}

	return tf.File.Read(data)
}

func (tf *TorrentFile) Seek(offset int64, whence int) (int64, error) {
	seekingOffset := offset

	switch whence {
	case os.SEEK_CUR:
		currentOffset, err := tf.File.Seek(0, os.SEEK_CUR)
		if err != nil {
			return currentOffset, err
		}
		seekingOffset += currentOffset
		break
	case os.SEEK_END:
		seekingOffset = tf.fileSize - offset
		break
	}

	tf.tfs.log.Infof("Seeking at %d...", seekingOffset)
	piece, _ := tf.pieceFromOffset(seekingOffset)
	if tf.hasPiece(piece) == false {
		tf.tfs.log.Infof("We don't have piece %d, setting piece priorities", piece)
		piecesPriorities := libtorrent.NewStdVectorInt()
		defer libtorrent.DeleteStdVectorInt(piecesPriorities)
		curPiece := 0
		numPieces := tf.torrentInfo.NumPieces()
		for _ = 0; curPiece < piece; curPiece++ {
			piecesPriorities.PushBack(0)
		}
		for _ = 0; curPiece < numPieces; curPiece++ {
			piecesPriorities.PushBack(1)
		}
		tf.torrentHandle.PrioritizePieces(piecesPriorities)
	}

	return tf.File.Seek(offset, whence)
}

func (tf *TorrentFile) waitForPiece(piece int) error {
	if tf.hasPiece(piece) {
		return nil
	}

	tf.tfs.log.Infof("Waiting for piece %d", piece)

	pieceRefreshTicker := time.Tick(piecesRefreshDuration)
	removed, done := tf.removed.Listen()
	defer close(done)
	for tf.hasPiece(piece) == false {
		select {
		case <-removed:
			tf.tfs.log.Infof("Unable to wait for piece %d as file was closed", piece)
			return errors.New("File was closed.")
		case <-pieceRefreshTicker:
			continue
		}
	}
	return nil
}

func (tf *TorrentFile) pieceFromOffset(offset int64) (int, int) {
	piece := (tf.fileOffset + offset) / int64(tf.pieceLength)
	pieceOffset := (tf.fileOffset + offset) % int64(tf.pieceLength)
	return int(piece), int(pieceOffset)
}
