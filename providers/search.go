package providers

import (
	"sort"
	"sync"

	"github.com/op/go-logging"
	"github.com/i96751414/pulsar/bittorrent"
	"github.com/i96751414/pulsar/tmdb"
	"github.com/i96751414/pulsar/tvdb"
)

var DefaultTrackers = []string{
	"udp://open.demonii.com:1337",
	"udp://tracker.coppersurfer.tk:6969",
	"udp://tracker.leechers-paradise.org:6969",
	"udp://tracker.openbittorrent.com:80",
	"udp://exodus.desync.com:6969",
	"udp://tracker.pomf.se",
	"udp://tracker.blackunicorn.xyz:6969",
	"udp://tracker.publicbt.com:80",
	"udp://pow7.com:80/announce",
}

var log = logging.MustGetLogger("linkssearch")

func Search(searchers []Searcher, query string) []*bittorrent.Torrent {
	torrentsChan := make(chan *bittorrent.Torrent)
	go func() {
		wg := sync.WaitGroup{}
		for _, searcher := range searchers {
			wg.Add(1)
			go func(searcher Searcher) {
				defer wg.Done()
				for _, torrent := range searcher.SearchLinks(query) {
					torrentsChan <- torrent
				}
			}(searcher)
		}
		wg.Wait()
		close(torrentsChan)
	}()

	return processLinks(torrentsChan)
}

func SearchMovie(searchers []MovieSearcher, movie *tmdb.Movie) []*bittorrent.Torrent {
	torrentsChan := make(chan *bittorrent.Torrent)
	go func() {
		wg := sync.WaitGroup{}
		for _, searcher := range searchers {
			wg.Add(1)
			go func(searcher MovieSearcher) {
				defer wg.Done()
				for _, torrent := range searcher.SearchMovieLinks(movie) {
					torrentsChan <- torrent
				}
			}(searcher)
		}
		wg.Wait()
		close(torrentsChan)
	}()

	return processLinks(torrentsChan)
}

func SearchEpisode(searchers []EpisodeSearcher, show *tvdb.Show, episode *tvdb.Episode) []*bittorrent.Torrent {
	torrentsChan := make(chan *bittorrent.Torrent)
	go func() {
		wg := sync.WaitGroup{}
		for _, searcher := range searchers {
			wg.Add(1)
			go func(searcher EpisodeSearcher) {
				defer wg.Done()
				for _, torrent := range searcher.SearchEpisodeLinks(show, episode) {
					torrentsChan <- torrent
				}
			}(searcher)
		}
		wg.Wait()
		close(torrentsChan)
	}()

	return processLinks(torrentsChan)
}

func processLinks(torrentsChan chan *bittorrent.Torrent) []*bittorrent.Torrent {
	trackers := map[string]*bittorrent.Tracker{}
	torrentsMap := map[string]*bittorrent.Torrent{}

	torrents := make([]*bittorrent.Torrent, 0)

	log.Info("Resolving torrent files...")
	wg := sync.WaitGroup{}
	for torrent := range torrentsChan {
		torrents = append(torrents, torrent)
		wg.Add(1)
		go func(torrent *bittorrent.Torrent) {
			defer wg.Done()
			if err := torrent.Resolve(); err != nil {
				log.Error("Unable to resolve .torrent file at: %s", torrent.URI)
			}
		}(torrent)
	}
	wg.Wait()

	for _, torrent := range torrents {
		if torrent.InfoHash == "" { // ignore torrents whose infohash is empty
			log.Error("Infohash is empty for %s\n", torrent.URI)
			continue
		}
		if existingTorrent, exists := torrentsMap[torrent.InfoHash]; exists {
			existingTorrent.Trackers = append(existingTorrent.Trackers, torrent.Trackers...)
			if torrent.Resolution > existingTorrent.Resolution {
				existingTorrent.Resolution = torrent.Resolution
			}
			if torrent.VideoCodec > existingTorrent.VideoCodec {
				existingTorrent.VideoCodec = torrent.VideoCodec
			}
			if torrent.AudioCodec > existingTorrent.AudioCodec {
				existingTorrent.AudioCodec = torrent.AudioCodec
			}
			if torrent.RipType > existingTorrent.RipType {
				existingTorrent.RipType = torrent.RipType
			}
			if torrent.SceneRating > existingTorrent.SceneRating {
				existingTorrent.SceneRating = torrent.SceneRating
			}
		} else {
			torrentsMap[torrent.InfoHash] = torrent
		}
		for _, tracker := range torrent.Trackers {
			bTracker, err := bittorrent.NewTracker(tracker)
			if err != nil {
				continue
			}
			trackers[bTracker.URL.Host] = bTracker
		}
	}

	for _, trackerUrl := range DefaultTrackers {
		tracker, _ := bittorrent.NewTracker(trackerUrl)
		trackers[tracker.URL.Host] = tracker
	}

	torrents = make([]*bittorrent.Torrent, 0, len(torrentsMap))
	for _, torrent := range torrentsMap {
		torrents = append(torrents, torrent)
	}

	log.Info("Received %d links.\n", len(torrents))

	if len(torrents) == 0 {
		return torrents
	}

	log.Info("Scraping torrent metrics from %d trackers...\n", len(trackers))
	scrapeResults := make(chan []bittorrent.ScrapeResponseEntry)
	go func() {
		wg := sync.WaitGroup{}
		for _, tracker := range trackers {
			wg.Add(1)
			go func(tracker *bittorrent.Tracker) {
				defer wg.Done()
				if err := tracker.Connect(); err != nil {
					log.Info("Tracker %s is not available because: %s\n", tracker, err)
					return
				}
				scrapeResults <- tracker.Scrape(torrents)
			}(tracker)
		}
		wg.Wait()
		close(scrapeResults)
	}()

	for results := range scrapeResults {
		for i, result := range results {
			if int64(result.Seeders) > torrents[i].Seeds {
				torrents[i].Seeds = int64(result.Seeders)
			}
			if int64(result.Leechers) > torrents[i].Peers {
				torrents[i].Peers = int64(result.Leechers)
			}
		}
	}

	sort.Sort(sort.Reverse(BySeeds(torrents)))
	log.Info("Sorted torrent candidates:\n")
	for _, torrent := range torrents {
		log.Info("%s S:%d P:%d", torrent.Name, torrent.Seeds, torrent.Peers)
	}

	return torrents
}
