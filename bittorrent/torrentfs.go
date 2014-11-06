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
	"github.com/steeve/libtorrent-go"
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
	torrentHandle     libtorrent.Torrent_handle
	torrentInfo       libtorrent.Torrent_info
	fileEntry         libtorrent.File_entry
	fileEntryIdx      int
	pieceLength       int
	fileOffset        int64
	fileSize          int64
	piecesMx          sync.RWMutex
	pieces            Bitfield
	piecesLastUpdated time.Time
	lastStatus        libtorrent.Torrent_status
	removed           chan interface{}
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
		tfs.log.Error("Unable to unlock file because: %s", err)
	}

	tfs.log.Info("Opening %s", name)
	// NB: this does NOT return a pointer to vector, no need to free!
	torrentsVector := tfs.service.Session.Get_torrents()
	torrentsVectorSize := int(torrentsVector.Size())
	for i := 0; i < torrentsVectorSize; i++ {
		torrentHandle := torrentsVector.Get(i)
		if torrentHandle.Is_valid() == false {
			continue
		}
		torrentInfo := torrentHandle.Torrent_file()
		numFiles := torrentInfo.Num_files()
		for j := 0; j < numFiles; j++ {
			fe := torrentInfo.File_at(j)
			if name[1:] == fe.GetPath() {
				tfs.log.Info("%s belongs to torrent %s", name, torrentHandle.Status(uint(libtorrent.Torrent_handleQuery_name)).GetName())
				return NewTorrentFile(file, tfs, torrentHandle, torrentInfo, fe, j)
			}
		}
		defer libtorrent.DeleteTorrent_info(torrentInfo)
	}
	return file, err
}

func NewTorrentFile(file *os.File, tfs *TorrentFS, torrentHandle libtorrent.Torrent_handle, torrentInfo libtorrent.Torrent_info, fileEntry libtorrent.File_entry, fileEntryIdx int) (*TorrentFile, error) {
	tf := &TorrentFile{
		File:          file,
		tfs:           tfs,
		torrentHandle: torrentHandle,
		torrentInfo:   torrentInfo,
		fileEntry:     fileEntry,
		fileEntryIdx:  fileEntryIdx,
		pieceLength:   torrentInfo.Piece_length(),
		fileOffset:    fileEntry.GetOffset(),
		fileSize:      fileEntry.GetSize(),
		removed:       make(chan interface{}),
	}
	go tf.consumeAlerts()

	return tf, nil
}

func (tf *TorrentFile) consumeAlerts() {
	alerts, done := tf.tfs.service.Alerts()
	defer close(done)
	for alert := range alerts {
		switch alert.Xtype() {
		case libtorrent.Torrent_removed_alertAlert_type:
			removedAlert := libtorrent.SwigcptrTorrent_alert(alert.Swigcptr())
			if removedAlert.GetHandle().Equal(tf.torrentHandle) {
				close(tf.removed)
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
		tf.lastStatus = tf.torrentHandle.Status(uint(libtorrent.Torrent_handleQuery_pieces))
		if tf.lastStatus.GetState() > libtorrent.Torrent_statusSeeding {
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
	close(tf.removed)
	libtorrent.DeleteTorrent_info(tf.torrentInfo)
	return tf.File.Close()
}

func (tf *TorrentFile) Read(data []byte) (int, error) {
	currentOffset, err := tf.File.Seek(0, os.SEEK_CUR)
	if err != nil {
		return 0, err
	}
	// tf.tfs.log.Info("About to read from file at %d for %d\n", currentOffset, len(data))
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

	tf.tfs.log.Info("Seeking at %d...", seekingOffset)
	piece, _ := tf.pieceFromOffset(seekingOffset)
	if tf.hasPiece(piece) == false {
		tf.tfs.log.Info("We don't have piece %d, setting piece priorities", piece)
		piecesPriorities := libtorrent.NewStd_vector_int()
		defer libtorrent.DeleteStd_vector_int(piecesPriorities)
		curPiece := 0
		numPieces := tf.torrentInfo.Num_pieces()
		for _ = 0; curPiece < piece; curPiece++ {
			piecesPriorities.Add(0)
		}
		for _ = 0; curPiece < numPieces; curPiece++ {
			piecesPriorities.Add(1)
		}
		tf.torrentHandle.Prioritize_pieces(piecesPriorities)
	}

	return tf.File.Seek(offset, whence)
}

func (tf *TorrentFile) waitForPiece(piece int) error {
	if tf.hasPiece(piece) {
		return nil
	}

	tf.tfs.log.Info("Waiting for piece %d", piece)

	pieceRefreshTicker := time.Tick(piecesRefreshDuration)
	for tf.hasPiece(piece) == false {
		select {
		case <-tf.removed:
			tf.tfs.log.Info("Unable to wait for piece %d as file was closed", piece)
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
