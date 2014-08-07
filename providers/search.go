package providers

import (
	"sort"
	"sync"

	"github.com/op/go-logging"
	"github.com/steeve/pulsar/bittorrent"
	"github.com/steeve/pulsar/tmdb"
	"github.com/steeve/pulsar/trakt"
)

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

func SearchEpisode(searchers []EpisodeSearcher, episode *trakt.ShowEpisode) []*bittorrent.Torrent {
	torrentsChan := make(chan *bittorrent.Torrent)
	go func() {
		wg := sync.WaitGroup{}
		for _, searcher := range searchers {
			wg.Add(1)
			go func(searcher EpisodeSearcher) {
				defer wg.Done()
				for _, torrent := range searcher.SearchEpisodeLinks(episode) {
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
	log := logging.MustGetLogger("LinksSearch")

	trackers := map[string]*bittorrent.Tracker{}
	torrentsMap := map[string]*bittorrent.Torrent{}

	for torrent := range torrentsChan {
		if existingTorrent, exists := torrentsMap[torrent.InfoHash]; exists {
			if existingTorrent.Resolution < torrent.Resolution {
				existingTorrent.Resolution = torrent.Resolution
			}
			if existingTorrent.VideoCodec < torrent.VideoCodec {
				existingTorrent.VideoCodec = torrent.VideoCodec
			}
			if existingTorrent.AudioCodec < torrent.AudioCodec {
				existingTorrent.AudioCodec = torrent.AudioCodec
			}
			if existingTorrent.RipType < torrent.RipType {
				existingTorrent.RipType = torrent.RipType
			}
			if existingTorrent.SceneRating < torrent.SceneRating {
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

	defaultTrackers := []string{
		"udp://open.demonii.com:1337/announce",
		"udp://tracker.publicbt.com:80",
		"udp://tracker.openbittorrent.com:80",
		"udp://pow7.com:80/announce",
	}
	for _, trackerUrl := range defaultTrackers {
		tracker, _ := bittorrent.NewTracker(trackerUrl)
		trackers[tracker.URL.Host] = tracker
	}

	torrents := make([]*bittorrent.Torrent, 0, len(torrentsMap))
	for _, torrent := range torrentsMap {
		torrents = append(torrents, torrent)
	}

	log.Info("Received %d links.\n", len(torrents))

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
			if int(result.Seeders) > torrents[i].Seeds {
				torrents[i].Seeds = int(result.Seeders)
			}
			if int(result.Leechers) > torrents[i].Peers {
				torrents[i].Peers = int(result.Leechers)
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
