package bittorrent

import (
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/op/go-logging"
	"github.com/steeve/libtorrent-go"
)

type TorrentFS struct {
	http.Dir
	service *BTService
	log     *logging.Logger
}

type TorrentFile struct {
	http.File
	tfs           *TorrentFS
	torrentHandle libtorrent.Torrent_handle
	torrentInfo   libtorrent.Torrent_info
	fileEntry     libtorrent.File_entry
	fileEntryIdx  int
}

func NewTorrentFS(service *BTService, path string) *TorrentFS {
	return &TorrentFS{
		service: service,
		log:     logging.MustGetLogger("torrentfs"),
		Dir:     http.Dir(path),
	}
}

func (tfs *TorrentFS) Open(name string) (http.File, error) {
	file, err := tfs.Dir.Open(name)
	if err != nil {
		return nil, err
	}

	tfs.log.Info("Opening %s", name)
	torrentsVector := tfs.service.Session.Get_torrents()
	for i := 0; i < int(torrentsVector.Size()); i++ {
		torrentHandle := torrentsVector.Get(i)
		if torrentHandle.Is_valid() == false {
			continue
		}
		torrentInfo := torrentHandle.Torrent_file()
		for j := 0; j < torrentInfo.Num_files(); j++ {
			fe := torrentInfo.File_at(j)
			if name[1:] == fe.GetPath() {
				tfs.log.Info("%s belongs to torrent %s", name, torrentHandle.Status().GetName())
				return NewTorrentFile(file, tfs, torrentHandle, torrentInfo, fe, j)
			}
		}
		defer libtorrent.DeleteTorrent_info(torrentInfo)
	}
	return file, err
}

func NewTorrentFile(file http.File, tfs *TorrentFS, torrentHandle libtorrent.Torrent_handle, torrentInfo libtorrent.Torrent_info, fileEntry libtorrent.File_entry, fileEntryIdx int) (*TorrentFile, error) {
	tf := &TorrentFile{
		File:          file,
		tfs:           tfs,
		torrentHandle: torrentHandle,
		torrentInfo:   torrentInfo,
		fileEntry:     fileEntry,
		fileEntryIdx:  fileEntryIdx,
	}
	runtime.SetFinalizer(tf, func(tf *TorrentFile) {
		libtorrent.DeleteTorrent_info(tf.torrentInfo)
	})
	return tf, nil
}

func (tf *TorrentFile) Read(data []byte) (int, error) {
	currentOffset, err := tf.File.Seek(0, os.SEEK_CUR)
	if err != nil {
		return -1, err
	}
	piece, _ := tf.pieceFromOffset(currentOffset + int64(len(data)))
	tf.waitForPiece(piece)
	return tf.File.Read(data)
}

func (tf *TorrentFile) Seek(offset int64, whence int) (int64, error) {
	seekingOffset := offset

	switch whence {
	case os.SEEK_CUR:
		currentOffset, err := tf.File.Seek(0, os.SEEK_CUR)
		if err != nil {
			return -1, err
		}
		seekingOffset += currentOffset
		break
	case os.SEEK_END:
		seekingOffset = tf.Size() - offset
		break
	}

	piece, _ := tf.pieceFromOffset(seekingOffset)

	piecesPriorities := libtorrent.NewStd_vector_int()
	defer libtorrent.DeleteStd_vector_int(piecesPriorities)
	curPiece := 0
	for curPiece < piece {
		piecesPriorities.Add(0)
		curPiece++
	}
	for curPiece < tf.torrentInfo.Num_pieces() {
		piecesPriorities.Add(1)
		curPiece++
	}
	tf.torrentHandle.Prioritize_pieces(piecesPriorities)

	tf.waitForPiece(piece)

	return tf.File.Seek(offset, whence)
}

func (tf *TorrentFile) waitForPiece(piece int) {
	if tf.torrentHandle.Piece_priority(piece).(int) > 0 {
		for tf.torrentHandle.Have_piece(piece) == false {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (tf *TorrentFile) pieceFromOffset(offset int64) (int, int) {
	pieceLength := int64(tf.torrentInfo.Piece_length())
	piece := int((tf.fileEntry.GetOffset() + offset) / pieceLength)
	pieceOffset := int((tf.fileEntry.GetOffset() + offset) % pieceLength)
	return piece, pieceOffset
}

func (tf *TorrentFile) Size() int64 {
	return tf.fileEntry.GetSize()
}

func (tf *TorrentFile) ModTime() time.Time {
	return time.Unix(int64(tf.fileEntry.GetMtime()), 0)
}
