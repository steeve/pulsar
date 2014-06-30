package streamer

import "github.com/steeve/pulsar/bittorrent"

func AddTorrent(torrent *bittorrent.Torrent) {
	// torrentParams := libtorrent.NewAdd_torrent_params()
	// defer libtorrent.DeleteAdd_torrent_params(torrentParams)

	// fileUri, _ := url.Parse(torrent.URI)
	// if fileUri.Scheme == "file" {
	// 	log.Printf("Opening local file %s\n", fileUri.Path)
	// 	torrentInfo := libtorrent.NewTorrent_info(fileUri.Path)
	// 	defer libtorrent.DeleteTorrent_info(torrentInfo)
	// 	torrentParams.SetTi(torrentInfo)
	// } else {
	// 	log.Println("Fetching link")
	// 	torrentParams.SetUrl(torrent.URI)
	// }

	// log.Println("Setting save path")
	// torrentParams.SetSave_path(streamer.config.DownloadPath)
	// torrentHandle := instance.session.Add_torrent(torrentParams)
	// torrentHandle.
}

func RemoveTorrent(torrentIdx int) {

}
