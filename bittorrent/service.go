package bittorrent

import (
	"os"
	"io"
	"fmt"
	"time"
	"strings"
	"runtime"
	"net/url"
	"io/ioutil"
	"encoding/hex"
	"path/filepath"

	"github.com/op/go-logging"
	"github.com/scakemyer/libtorrent-go"
	"github.com/scakemyer/quasar/broadcast"
	"github.com/scakemyer/quasar/util"
	"github.com/scakemyer/quasar/xbmc"
)

const (
	libtorrentAlertWaitTime = 1 // 1 second
)

const (
	ipToSDefault     = iota
	ipToSLowDelay    = 1 << iota
	ipToSReliability = 1 << iota
	ipToSThroughput  = 1 << iota
	ipToSLowCost     = 1 << iota
)

var dhtBootstrapNodes = []string{
	"router.bittorrent.com",
	"router.utorrent.com",
	"dht.transmissionbt.com",
	"dht.aelitis.com", // Vuze
}

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

var StatusStrings = []string{
	"Queued",
	"Checking",
	"Finding",
	"Buffering",
	"Finished",
	"Seeding",
	"Allocating",
	"Stalled",
}

const (
	ProxyTypeNone = iota
	ProxyTypeSocks4
	ProxyTypeSocks5
	ProxyTypeSocks5Password
	ProxyTypeSocksHTTP
	ProxyTypeSocksHTTPPassword
)

type ProxySettings struct {
	Hostname string
	Port     int
	Username string
	Password string
	Type     int
}

type BTConfiguration struct {
	BackgroundHandling  bool
	BufferSize          int
	MaxUploadRate       int
	MaxDownloadRate     int
	LimitAfterBuffering bool
	ConnectionsLimit    int
	SessionSave         int
	LowerListenPort     int
	UpperListenPort     int
	DownloadPath        string
	Proxy               *ProxySettings
}

type BTService struct {
	Session           libtorrent.Session
	config            *BTConfiguration
	log               *logging.Logger
	libtorrentLog     *logging.Logger
	alertsBroadcaster *broadcast.Broadcaster
	dialogProgressBG  *xbmc.DialogProgressBG
	closing           chan interface{}
}

type activeTorrent struct {
	torrentName       string
	progress          int
}

func NewBTService(config BTConfiguration) *BTService {
	s := &BTService{
		Session:           libtorrent.NewSession(),
		log:               logging.MustGetLogger("btservice"),
		libtorrentLog:     logging.MustGetLogger("libtorrent"),
		alertsBroadcaster: broadcast.NewBroadcaster(),
		config:            &config,
		closing:           make(chan interface{}),
	}

	s.configure()
	go s.saveResumeDataConsumer()
	go s.saveResumeDataLoop()
	go s.downloadProgress()
	go s.alertsConsumer()
	go s.logAlerts()

	if config.BackgroundHandling {
		go s.loadFastResumeFiles()
	}

	return s
}

func (s *BTService) Close() {
	s.log.Info("Stopping BT Services...")
	close(s.closing)
	libtorrent.DeleteSession(s.Session)
}

func (s *BTService) Reconfigure(config BTConfiguration) {
	s.stopServices()
	s.config = &config
	s.configure()
	s.Listen()
	s.startServices()
}

func (s *BTService) configure() {
	settings := s.Session.Settings()

	s.log.Info("Setting Session settings...")

	settings.SetUserAgent(util.UserAgent())

	settings.SetRequestTimeout(2)
	settings.SetPeerConnectTimeout(2)
	settings.SetStrictEndGameMode(true)
	settings.SetAnnounceToAllTrackers(true)
	settings.SetAnnounceToAllTiers(true)
	settings.SetConnectionSpeed(500)

	if s.config.ConnectionsLimit > 0 {
		settings.SetConnectionsLimit(s.config.ConnectionsLimit)
	}

	if s.config.LimitAfterBuffering == false {
		if s.config.MaxDownloadRate > 0 {
			s.log.Info("Rate limiting download to %dkb/s", s.config.MaxDownloadRate / 1024)
			settings.SetDownloadRateLimit(s.config.MaxDownloadRate)
		}
		if s.config.MaxUploadRate > 0 {
			s.log.Info("Rate limiting upload to %dkb/s", s.config.MaxUploadRate / 1024)
			// If we have an upload rate, use the nicer bittyrant choker
			settings.SetChokingAlgorithm(int(libtorrent.SessionSettingsBittyrantChoker))
			settings.SetUploadRateLimit(s.config.MaxUploadRate)
		}
	}

	settings.SetPeerTos(ipToSLowCost)
	settings.SetTorrentConnectBoost(500)
	settings.SetRateLimitIpOverhead(true)
	settings.SetNoAtimeStorage(true)
	settings.SetAnnounceDoubleNat(true)
	settings.SetPrioritizePartialPieces(false)
	settings.SetFreeTorrentHashes(true)
	settings.SetUseParoleMode(true)

	// Make sure the disk cache is not swapped out (useful for slower devices)
	settings.SetLockDiskCache(true)
	settings.SetDiskCacheAlgorithm(libtorrent.SessionSettingsLargestContiguous)

	// Prioritize people starting downloads
	settings.SetSeedChokingAlgorithm(int(libtorrent.SessionSettingsFastestUpload))

	// copied from qBitorrent at
	// https://github.com/qbittorrent/qBittorrent/blob/master/src/qtlibtorrent/qbtsession.cpp
	settings.SetUpnpIgnoreNonrouters(true)
	settings.SetLazyBitfields(true)
	settings.SetStopTrackerTimeout(1)
	settings.SetAutoScrapeInterval(1200)    // 20 minutes
	settings.SetAutoScrapeMinInterval(900) // 15 minutes
	settings.SetIgnoreLimitsOnLocalNetwork(true)
	settings.SetRateLimitUtp(true)
	settings.SetMixedModeAlgorithm(int(libtorrent.SessionSettingsPreferTcp))

	setPlatformSpecificSettings(settings)

	s.Session.SetSettings(settings)

	// Add all the libtorrent extensions
	s.Session.AddExtensions()

	s.log.Info("Setting Encryption settings...")
	encryptionSettings := libtorrent.NewPeSettings()
	defer libtorrent.DeletePeSettings(encryptionSettings)
	encryptionSettings.SetOutEncPolicy(byte(libtorrent.PeSettingsForced))
	encryptionSettings.SetInEncPolicy(byte(libtorrent.PeSettingsForced))
	encryptionSettings.SetAllowedEncLevel(byte(libtorrent.PeSettingsBoth))
	encryptionSettings.SetPreferRc4(true)
	s.Session.SetPeSettings(encryptionSettings)

	if s.config.Proxy != nil {
		s.log.Info("Setting Proxy settings...")
		proxy := libtorrent.NewProxySettings()
		defer libtorrent.DeleteProxySettings(proxy)
		proxy.SetHostname(s.config.Proxy.Hostname)
		proxy.SetPort(uint16(s.config.Proxy.Port))
		proxy.SetUsername(s.config.Proxy.Username)
		proxy.SetPassword(s.config.Proxy.Password)
		proxy.SetType(byte(s.config.Proxy.Type))
		proxy.SetProxyHostnames(true)
		proxy.SetProxyPeerConnections(true)
		s.Session.SetProxy(proxy)
	}
}

func (s *BTService) Listen() {
	errCode := libtorrent.NewErrorCode()
	defer libtorrent.DeleteErrorCode(errCode)
	ports := libtorrent.NewStdPairIntInt(s.config.LowerListenPort, s.config.UpperListenPort)
	defer libtorrent.DeleteStdPairIntInt(ports)
	s.Session.ListenOn(ports, errCode)
}

func (s *BTService) WriteState(f io.Writer) error {
	entry := libtorrent.NewEntry()
	defer libtorrent.DeleteEntry(entry)
	s.Session.SaveState(entry, 0xFFFF)
	_, err := f.Write([]byte(libtorrent.Bencode(entry)))
	return err
}

func (s *BTService) LoadState(f io.Reader) error {
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	entry := libtorrent.NewLazyEntry()
	defer libtorrent.DeleteLazyEntry(entry)
	libtorrent.LazyBdecode(string(data), entry)
	s.Session.LoadState(entry)
	return nil
}

func (s *BTService) startServices() {
	s.log.Info("Starting DHT...")
	for _, node := range dhtBootstrapNodes {
		pair := libtorrent.NewStdPairStringInt(node, 6881)
		defer libtorrent.DeleteStdPairStringInt(pair)
		s.Session.AddDhtRouter(pair)
	}
	s.Session.StartDht()

	s.log.Info("Starting LSD...")
	s.Session.StartLsd()

	s.log.Info("Starting UPNP...")
	s.Session.StartUpnp()

	s.log.Info("Starting NATPMP...")
	s.Session.StartNatpmp()
}

func (s *BTService) stopServices() {
	if s.dialogProgressBG != nil {
		s.dialogProgressBG.Close()
	}

	s.log.Info("Stopping DHT...")
	s.Session.StopDht()

	s.log.Info("Stopping LSD...")
	s.Session.StopLsd()

	s.log.Info("Stopping UPNP...")
	s.Session.StopUpnp()

	s.log.Info("Stopping NATPMP...")
	s.Session.StopNatpmp()
}

func (s *BTService) saveResumeDataLoop() {
	saveResumeWait := time.NewTicker(time.Duration(s.config.SessionSave) * time.Second)
	defer saveResumeWait.Stop()

	for {
		select {
		case <-saveResumeWait.C:
			torrentsVector := s.Session.GetTorrents()
			torrentsVectorSize := int(torrentsVector.Size())

			for i := 0; i < torrentsVectorSize; i++ {
				torrentHandle := torrentsVector.Get(i)
				if torrentHandle.IsValid() == false {
					continue
				}

				status := torrentHandle.Status()
				if status.GetHasMetadata() == false || status.GetNeedSaveResume() == false {
					continue
				}

				torrentHandle.SaveResumeData(1)
			}
		}
	}
}

func (s *BTService) saveResumeDataConsumer() {
	alerts, alertsDone := s.Alerts()
	defer close(alertsDone)

	for {
		select {
		case alert, ok := <-alerts:
			if !ok { // was the alerts channel closed?
				return
			}
			switch alert.Type() {
			case libtorrent.SaveResumeDataAlertAlertType:
				saveResumeData := libtorrent.SwigcptrSaveResumeDataAlert(alert.Swigcptr())
				torrentHandle := saveResumeData.GetHandle()
				torrentStatus := torrentHandle.Status(uint(libtorrent.TorrentHandleQuerySavePath) | uint(libtorrent.TorrentHandleQueryName))
				shaHash := torrentStatus.GetInfoHash().ToString()
				infoHash := hex.EncodeToString([]byte(shaHash))
				torrentName := torrentStatus.GetName()
				entry := saveResumeData.ResumeData()
				bEncoded := []byte(libtorrent.Bencode(entry))

				s.log.Info("Saving resume data for %s to %s.fastresume", torrentName, infoHash)
				path := filepath.Join(s.config.DownloadPath, fmt.Sprintf("%s.fastresume", infoHash))
				ioutil.WriteFile(path, bEncoded, 0644)
				break
			}
		}
	}
}

func (s *BTService) loadFastResumeFiles() error {
	pattern := filepath.Join(s.config.DownloadPath, "*.fastresume")
	files, _ := filepath.Glob(pattern)
	for _, fastResumeFile := range files {
		torrentParams := libtorrent.NewAddTorrentParams()
		defer libtorrent.DeleteAddTorrentParams(torrentParams)

		s.log.Info("Loading fast resume file %s", fastResumeFile)

		hashFromPath := strings.Split(strings.TrimSuffix(fastResumeFile, ".fastresume"), string(os.PathSeparator))
		infoHash := hashFromPath[len(hashFromPath) - 1]
		uri := fmt.Sprintf("magnet:?xt=urn:btih:%s", infoHash)

		torrent := NewTorrent(uri)
		magnet := torrent.Magnet()
		infoHash = torrent.InfoHash
		boosters := url.Values{
			"tr": DefaultTrackers,
		}
		magnet += "&" + boosters.Encode()
		torrentParams.SetUrl(magnet)
		torrentParams.SetSavePath(s.config.DownloadPath)

		fastResumeData, err := ioutil.ReadFile(fastResumeFile)
		if err != nil {
			return err
		}
		fastResumeVector := libtorrent.NewStdVectorChar()
		for _, c := range fastResumeData {
			fastResumeVector.PushBack(c)
		}
		torrentParams.SetResumeData(fastResumeVector)

		torrentHandle := s.Session.AddTorrent(torrentParams)

		if torrentHandle == nil {
			return fmt.Errorf("Unable to add torrent from %s", fastResumeFile)
		}
	}

	return nil
}

func (s *BTService) downloadProgress() {
	rotateTicker := time.NewTicker(5 * time.Second)
	defer rotateTicker.Stop()

	showNext := 0
	for {
		select {
		case <-rotateTicker.C:
			if s.Session.IsPaused() && s.dialogProgressBG != nil {
				s.dialogProgressBG.Close()
				s.dialogProgressBG = nil
				continue
			}

			torrentsVector := s.Session.GetTorrents()
			torrentsVectorSize := int(torrentsVector.Size())
			totalProgress := 0
			activeTorrents := make([]*activeTorrent, 0)

			for i := 0; i < torrentsVectorSize; i++ {
				torrentHandle := torrentsVector.Get(i)
				if torrentHandle.IsValid() == false {
					continue
				}

				torrentStatus := torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName))
				if torrentStatus.GetHasMetadata() == false  || torrentStatus.GetPaused() || s.Session.IsPaused() {
					continue
				}

				torrentName := torrentStatus.GetName()
				progress := int(float64(torrentStatus.GetProgress()) * 100)

				if progress < 100 {
					activeTorrents = append(activeTorrents, &activeTorrent{
						torrentName: torrentName,
						progress: progress,
					})
					totalProgress += progress
				}
			}

			activeDownloads := len(activeTorrents)
			if activeDownloads > 0 {
				showProgress := totalProgress / activeDownloads
				showTorrent := "Total"
				if showNext >= activeDownloads {
					showNext = 0
				} else {
					showProgress = activeTorrents[showNext].progress
					showTorrent = activeTorrents[showNext].torrentName
					if len(showTorrent) > 32 {
						showTorrent = showTorrent[:32]
					}
					showNext += 1
				}
				if s.dialogProgressBG == nil {
					s.dialogProgressBG = xbmc.NewDialogProgressBG("Quasar", "")
				}
				s.dialogProgressBG.Update(showProgress, fmt.Sprintf("Quasar - %s", showTorrent))
			} else if s.dialogProgressBG != nil {
				s.dialogProgressBG.Close()
				s.dialogProgressBG = nil
			}
		}
	}
}

func (s *BTService) alertsConsumer() {
	s.Session.SetAlertMask(uint(libtorrent.AlertStatusNotification |
		libtorrent.AlertStorageNotification))

	defer s.alertsBroadcaster.Close()

	ltOneSecond := libtorrent.Seconds(libtorrentAlertWaitTime)
	s.log.Info("Consuming alerts...")
	for {
		select {
		case <-s.closing:
			s.log.Info("Closing all alert channels...")
			return
		default:
			if s.Session.WaitForAlert(ltOneSecond).Swigcptr() == 0 {
				continue
			}
			alert := &Alert{s.Session.PopAlert()}
			runtime.SetFinalizer(alert, func(alert *Alert) {
				libtorrent.DeleteAlert(*alert)
			})
			s.alertsBroadcaster.Broadcast(alert)
		}
	}
}

func (s *BTService) Alerts() (<-chan *Alert, chan<- interface{}) {
	c, done := s.alertsBroadcaster.Listen()
	ac := make(chan *Alert)
	go func() {
		for v := range c {
			ac <- v.(*Alert)
		}
	}()
	return ac, done
}

func (s *BTService) logAlerts() {
	alerts, _ := s.Alerts()
	for alert := range alerts {
		alertCategory := alert.Category()
		if alertCategory&int(libtorrent.AlertErrorNotification) != 0 {
			s.libtorrentLog.Error("%s: %s", alert.What(), alert.Message())
		} else if alertCategory&int(libtorrent.AlertDebugNotification) != 0 {
			s.libtorrentLog.Debug("%s: %s", alert.What(), alert.Message())
		} else if alertCategory&int(libtorrent.AlertPerformanceWarning) != 0 {
			s.libtorrentLog.Warning("%s: %s", alert.What(), alert.Message())
		} else {
			s.libtorrentLog.Notice("%s: %s", alert.What(), alert.Message())
		}
	}
}
