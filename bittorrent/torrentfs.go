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
	// fp            *os.File
	// stat          os.FileInfo
}

func NewTorrentFS(service *BTService, path string) *TorrentFS {
	return &TorrentFS{
		service: service,
		log:     logging.MustGetLogger("TorrentFS"),
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
		torrentInfo := torrentHandle.Torrent_file()
		for j := 0; j < torrentInfo.Num_files(); j++ {
			fe := torrentInfo.File_at(j)
			if name[1:] == fe.GetPath() {
				tfs.log.Info("FOUND A TORRENT FILE!!!")
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
	if tf.torrentHandle.Have_piece(piece) == false {
		for i := 0; i < piece; i++ {
			tf.torrentHandle.Piece_priority(i, 0)
		}
	}
	for i := piece; i < tf.torrentInfo.Num_pieces(); i++ {
		tf.torrentHandle.Piece_priority(i, 1)
	}
	tf.waitForPiece(piece)

	return tf.File.Seek(offset, whence)
}

func (tf *TorrentFile) waitForPiece(piece int) {
	for tf.torrentHandle.Piece_priority(piece).(int) > 0 && tf.torrentHandle.Have_piece(piece) == false {
		time.Sleep(500 * time.Millisecond)
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

// 	savePath := tfs.btp.torrentHandle.Status().GetSave_path()
// 	files := tfs.btp.torrentFile.Files()
// 	for i := 0; i < tfs.btp.torrentFile.Num_files(); i++ {
// 		fileEntry := files.At(i)
// 		feAbsName, _ := filepath.Abs(path.Join(savePath, fileEntry.GetPath()))
// 		if feAbsName == tf.name {
// 			tf.fe = fileEntry
// 			tf.feIdx = i
// 			break
// 		}
// 	}
// 	if tf.fe == nil {
// 		return nil, fmt.Errorf("unable to find file %s", name)
// 	}
// 	if fp, err := os.Open(name); err == nil {
// 		tf.fp = fp
// 	} else {
// 		return nil, err
// 	}
// 	return tf, nil
// }

// func NewTorrentFileFromFileEntry(tfs *TorrentFS, fe libtorrent.File_entry) (*TorrentFile, error) {
// 	tf := &TorrentFile{
// 		tfs: tfs,
// 		btp: tfs.btp,
// 	}

// 	savePath := tfs.btp.torrentHandle.Status().GetSave_path()
// 	files := tfs.btp.torrentFile.Files()
// 	for i := 0; i < tfs.btp.torrentFile.Num_files(); i++ {
// 		fileEntry := files.At(i)
// 		feAbsName, _ := filepath.Abs(path.Join(savePath, fileEntry.GetPath()))
// 		if feAbsName == tf.name {
// 			tf.fe = fileEntry
// 			tf.feIdx = i
// 			break
// 		}
// 	}
// 	if tf.fe == nil {
// 		return nil, fmt.Errorf("unable to find file %s", name)
// 	}
// 	if fp, err := os.Open(name); err == nil {
// 		tf.fp = fp
// 	} else {
// 		return nil, err
// 	}
// 	return tf, nil
// }

// func (tf *TorrentFile) Close() error {
// 	return tf.fp.Close()
// }

// func (tf *TorrentFile) Stat() (os.FileInfo, error) {
// 	return tf, nil
// }

// func (tf *TorrentFile) pieceFromOffset(offset int64) (int, int) {
// 	if offset > tf.Size() {
// 		offset = tf.Size()
// 	}
// 	pieceLength := int64(tf.btp.torrentFile.Piece_length())
// 	piece := int((tf.Offset() + offset) / pieceLength)
// 	pieceOffset := int((tf.Offset() + offset) % pieceLength)
// 	return piece, pieceOffset
// }

// func (tf *TorrentFile) waitForPiece(piece int) {
// 	for tf.btp.torrentHandle.Have_piece(piece) == false && tf.btp.torrentHandle.Piece_priority(piece).(int) > 0 {
// 		time.Sleep(100 * time.Millisecond)
// 	}
// }

// func (tf *TorrentFile) Readdir(count int) ([]os.FileInfo, error) {
// 	savePath := tf.btp.torrentHandle.Status().GetSave_path()
// 	totalFiles := tf.btp.torrentFile.Num_files()
// 	files := make([]os.FileInfo, 0, totalFiles)
// 	torrentFiles := tf.btp.torrentFile.Files()
// 	for i := 0; i < totalFiles; i++ {
// 		fileEntry := torrentFiles.At(i)
// 		feAbsName, _ := filepath.Abs(path.Join(savePath, fileEntry.GetPath()))
// 		torrentFile, _ := NewTorrentFile(tf.tfs, feAbsName)
// 		files = append(files, torrentFile)
// 	}
// 	return files, nil
// }

// func (tf *TorrentFile) Read(data []byte) (int, error) {
// 	currentOffset, err := tf.fp.Seek(0, os.SEEK_CUR)
// 	if err != nil {
// 		return -1, err
// 	}

// 	piece, _ := tf.pieceFromOffset(currentOffset + int64(len(data)))
// 	tf.waitForPiece(piece)
// 	return tf.fp.Read(data)
// }

// func (tf *TorrentFile) Seek(offset int64, whence int) (int64, error) {
// 	switch whence {
// 	case os.SEEK_CUR:
// 		currentOffset, err := tf.fp.Seek(0, os.SEEK_CUR)
// 		if err != nil {
// 			return -1, err
// 		}
// 		offset += currentOffset
// 		break
// 	case os.SEEK_END:
// 		offset = tf.Size() - offset
// 		break
// 	}
// 	piece, _ := tf.pieceFromOffset(offset)
// 	tf.waitForPiece(piece)
// 	// startPiece, _ := tf.Pieces()
// 	// for i := startPiece; i < piece; i++ {
// 	// 	tf.tfs.th.Piece_priority(i, 0)
// 	// }
// 	// tf.tfs.th.Piece_priority(piece, 7)

// 	return tf.fp.Seek(offset, os.SEEK_SET)
// }

// // os.FileInfo
// func (tf *TorrentFile) Name() string {
// 	stat, _ := os.Stat(tf.name)
// 	return stat.Name()
// }

// func (tf *TorrentFile) Mode() os.FileMode {
// 	stat, _ := os.Stat(tf.name)
// 	return stat.Mode()
// }

// func (tf *TorrentFile) ModTime() time.Time {
// 	return time.Unix(int64(tf.fe.GetMtime()), 0)
// }

// func (tf *TorrentFile) IsDir() bool {
// 	return strings.HasSuffix(tf.name, "/")
// }

// func (tf *TorrentFile) Sys() interface{} {
// 	return nil
// }

// // Specific to libtorrent
// func (tf *TorrentFile) Offset() int64 {
// 	return tf.fe.GetOffset()
// }

// func (tf *TorrentFile) SetPriority(priority int) {
// 	tf.tfs.log.Info("Setting priority %d to file %s\n", priority, tf.Name())
// 	tf.btp.torrentHandle.File_priority(tf.feIdx, priority)
// }

// // func (tf *TorrentFile) Pieces() (int, int) {
// // 	startPiece, _ := tf.pieceFromOffset(1)
// // 	endPiece, _ := tf.pieceFromOffset(tf.Size() - 1)
// // 	return startPiece, endPiece
// // }

// // func (tf *TorrentFile) CompletedPieces() int {
// // 	pieces := tf.tfs.th.Status().GetPieces()
// // 	startPiece, endPiece := tf.Pieces()
// // 	for i := startPiece; i <= endPiece; i++ {
// // 		if pieces.Get_bit(i) == false {
// // 			return i - startPiece
// // 		}
// // 	}
// // 	return endPiece - startPiece
// // }

// // func (tf *TorrentFile) ShowPieces() {
// // 	pieces := tf.tfs.th.Status().GetPieces()
// // 	startPiece, endPiece := tf.Pieces()
// // 	for i := startPiece; i <= endPiece; i++ {
// // 		if pieces.Get_bit(i) == false {
// // 			fmt.Printf("-")
// // 		} else {
// // 			fmt.Printf("#")
// // 		}
// // 	}
// // }
