package bittorrent

import (
	"os"
	"io"
	"fmt"
	"time"
	"strings"
	"strconv"
	"io/ioutil"
	"math/rand"
	"encoding/hex"
	"path/filepath"

	"github.com/op/go-logging"
	"github.com/dustin/go-humanize"
	"github.com/scakemyer/libtorrent-go"
	"github.com/scakemyer/quasar/broadcast"
	"github.com/scakemyer/quasar/diskusage"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/tmdb"
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
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://tracker.coppersurfer.tk:6969/announce",
	"udp://tracker.leechers-paradise.org:6969/announce",
	"udp://tracker.openbittorrent.com:80/announce",
	"udp://explodie.org:6969",
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
	ProxyTypeI2PSAM
)

type ProxySettings struct {
	Type     int
	Port     int
	Hostname string
	Username string
	Password string
}

type BTConfiguration struct {
	SpoofUserAgent      int
	BackgroundHandling  bool
	BufferSize          int
	MaxUploadRate       int
	MaxDownloadRate     int
	LimitAfterBuffering bool
	ConnectionsLimit    int
	SessionSave         int
	ShareRatioLimit     int
	SeedTimeRatioLimit  int
	SeedTimeLimit       int
	DisableDHT          bool
	DisableUPNP         bool
	EncryptionPolicy    int
	LowerListenPort     int
	UpperListenPort     int
	TunedStorage        bool
	DownloadPath        string
	TorrentsPath        string
	Proxy               *ProxySettings
}

type BTService struct {
	Session           libtorrent.Session
	config            *BTConfiguration
	log               *logging.Logger
	libtorrentLog     *logging.Logger
	alertsBroadcaster *broadcast.Broadcaster
	dialogProgressBG  *xbmc.DialogProgressBG
	packSettings      libtorrent.SettingsPack
	closing           chan interface{}
}

type activeTorrent struct {
	torrentName       string
	progress          int
}

type ResumeFile struct {
	InfoHash  string     `bencode:"info-hash"`
	Trackers  [][]string `bencode:"trackers"`
}

func NewBTService(config BTConfiguration) *BTService {
	s := &BTService{
		log:               logging.MustGetLogger("btservice"),
		libtorrentLog:     logging.MustGetLogger("libtorrent"),
		alertsBroadcaster: broadcast.NewBroadcaster(),
		config:            &config,
		closing:           make(chan interface{}),
	}

	if _, err := os.Stat(s.config.TorrentsPath); os.IsNotExist(err) {
		if err := os.Mkdir(s.config.TorrentsPath, 0755); err != nil{
			s.log.Error("Unable to create Torrents folder")
		}
	}

	s.configure()
	s.startServices()

	go s.saveResumeDataConsumer()
	go s.saveResumeDataLoop()
	go s.alertsConsumer()
	go s.logAlerts()

	tmdb.CheckApiKey()

	if config.BackgroundHandling {
		go s.loadTorrentFiles()
		go s.downloadProgress()
	}

	return s
}

func (s *BTService) Close() {
	s.log.Info("Stopping BT Services...")
	s.stopServices()
	close(s.closing)
	libtorrent.DeleteSession(s.Session)
}

func (s *BTService) Reconfigure(config BTConfiguration) {
	s.stopServices()
	s.config = &config
	s.configure()
	s.startServices()
	s.loadTorrentFiles()
}

func (s *BTService) configure() {
	settings := libtorrent.NewSettingsPack()
	s.Session = libtorrent.NewSession(settings, int(libtorrent.SessionHandleAddDefaultPlugins))

	s.log.Info("Applying session settings...")

	userAgent := util.UserAgent()
	if s.config.SpoofUserAgent > 0 {
		switch s.config.SpoofUserAgent {
		case 1:
			userAgent = ""
			break
		case 2:
			userAgent = "libtorrent (Rasterbar) 1.1.0"
			break
		case 3:
			userAgent = "BitTorrent 7.5.0"
			break
		case 4:
			userAgent = "BitTorrent 7.4.3"
			break
		case 5:
			userAgent = "µTorrent 3.4.9"
			break
		case 6:
			userAgent = "µTorrent 3.2.0"
			break
		case 7:
			userAgent = "µTorrent 2.2.1"
			break
		case 8:
			userAgent = "Transmission 2.92"
			break
		case 9:
			userAgent = "Deluge 1.3.6.0"
			break
		case 10:
			userAgent = "Deluge 1.3.12.0"
			break
		case 11:
			userAgent = "Vuze 5.7.3.0"
			break
		}
		if userAgent != "" {
			s.log.Infof("UserAgent: %s", userAgent)
		} else {
			s.log.Infof("UserAgent: libtorrent/%s", libtorrent.Version())
		}
	} else {
		s.log.Infof("UserAgent: %s", util.UserAgent())
	}

	if userAgent != "" {
		settings.SetStr(libtorrent.SettingByName("user_agent"), userAgent)
	}
	settings.SetInt(libtorrent.SettingByName("request_timeout"), 2)
	settings.SetInt(libtorrent.SettingByName("peer_connect_timeout"), 2)
	settings.SetBool(libtorrent.SettingByName("strict_end_game_mode"), true)
	settings.SetBool(libtorrent.SettingByName("announce_to_all_trackers"), true)
	settings.SetBool(libtorrent.SettingByName("announce_to_all_tiers"), true)
	settings.SetInt(libtorrent.SettingByName("connection_speed"), 500)
	settings.SetInt(libtorrent.SettingByName("connections_limit"), 0)
	settings.SetInt(libtorrent.SettingByName("download_rate_limit"), 0)
	settings.SetInt(libtorrent.SettingByName("upload_rate_limit"), 0)
	settings.SetInt(libtorrent.SettingByName("choking_algorithm"), 0)
	settings.SetInt(libtorrent.SettingByName("share_ratio_limit"), 0)
	settings.SetInt(libtorrent.SettingByName("seed_time_ratio_limit"), 0)
	settings.SetInt(libtorrent.SettingByName("seed_time_limit"), 0)
	settings.SetInt(libtorrent.SettingByName("peer_tos"), ipToSLowCost)
	settings.SetInt(libtorrent.SettingByName("torrent_connect_boost"), 0)
	settings.SetBool(libtorrent.SettingByName("rate_limit_ip_overhead"), true)
	settings.SetBool(libtorrent.SettingByName("no_atime_storage"), true)
	settings.SetBool(libtorrent.SettingByName("announce_double_nat"), true)
	settings.SetBool(libtorrent.SettingByName("prioritize_partial_pieces"), false)
	settings.SetBool(libtorrent.SettingByName("free_torrent_hashes"), true)
	settings.SetBool(libtorrent.SettingByName("use_parole_mode"), true)
	settings.SetInt(libtorrent.SettingByName("seed_choking_algorithm"), int(libtorrent.SettingsPackFastestUpload))
	settings.SetBool(libtorrent.SettingByName("upnp_ignore_nonrouters"), true)
	settings.SetBool(libtorrent.SettingByName("lazy_bitfields"), true)
	settings.SetInt(libtorrent.SettingByName("stop_tracker_timeout"), 1)
	settings.SetInt(libtorrent.SettingByName("auto_scrape_interval"), 1200)
	settings.SetInt(libtorrent.SettingByName("auto_scrape_min_interval"), 900)
	settings.SetBool(libtorrent.SettingByName("ignore_limits_on_local_network"), true)
	settings.SetBool(libtorrent.SettingByName("rate_limit_utp"), true)
	settings.SetInt(libtorrent.SettingByName("mixed_mode_algorithm"), int(libtorrent.SettingsPackPreferTcp))

	// For Android external storage / OS-mounted NAS setups
	if s.config.TunedStorage {
		settings.SetBool(libtorrent.SettingByName("use_read_cache"), true)
		settings.SetBool(libtorrent.SettingByName("coalesce_reads"), true)
		settings.SetBool(libtorrent.SettingByName("coalesce_writes"), true)
		settings.SetInt(libtorrent.SettingByName("max_queued_disk_bytes"), 10 * 1024 * 1024)
		settings.SetInt(libtorrent.SettingByName("cache_size"), -1)
	}

	if s.config.ConnectionsLimit > 0 {
		settings.SetInt(libtorrent.SettingByName("connections_limit"), s.config.ConnectionsLimit)
	} else {
		setPlatformSpecificSettings(settings)
	}

	if s.config.LimitAfterBuffering == false {
		if s.config.MaxDownloadRate > 0 {
			s.log.Infof("Rate limiting download to %dkB/s", s.config.MaxDownloadRate / 1024)
			settings.SetInt(libtorrent.SettingByName("download_rate_limit"), s.config.MaxDownloadRate)
		}
		if s.config.MaxUploadRate > 0 {
			s.log.Infof("Rate limiting upload to %dkB/s", s.config.MaxUploadRate / 1024)
			// If we have an upload rate, use the nicer bittyrant choker
			settings.SetInt(libtorrent.SettingByName("upload_rate_limit"), s.config.MaxUploadRate)
			settings.SetInt(libtorrent.SettingByName("choking_algorithm"), int(libtorrent.SettingsPackBittyrantChoker))
		}
	}

	if s.config.ShareRatioLimit > 0 {
		settings.SetInt(libtorrent.SettingByName("share_ratio_limit"), s.config.ShareRatioLimit)
	}
	if s.config.SeedTimeRatioLimit > 0 {
		settings.SetInt(libtorrent.SettingByName("seed_time_ratio_limit"), s.config.SeedTimeRatioLimit)
	}
	if s.config.SeedTimeLimit > 0 {
		settings.SetInt(libtorrent.SettingByName("seed_time_limit"), s.config.SeedTimeLimit)
	}

	s.log.Info("Applying encryption settings...")
	if s.config.EncryptionPolicy > 0 {
		policy := int(libtorrent.SettingsPackPeDisabled)
		level := int(libtorrent.SettingsPackPeBoth)
		preferRc4 := false

		if s.config.EncryptionPolicy == 2 {
			policy = int(libtorrent.SettingsPackPeForced)
			level = int(libtorrent.SettingsPackPeRc4)
			preferRc4 = true
		}

		settings.SetInt(libtorrent.SettingByName("out_enc_policy"), policy)
		settings.SetInt(libtorrent.SettingByName("in_enc_policy"), policy)
		settings.SetInt(libtorrent.SettingByName("allowed_enc_level"), level)
		settings.SetBool(libtorrent.SettingByName("prefer_rc4"), preferRc4)
	}

	if s.config.Proxy != nil {
		s.log.Info("Applying proxy settings...")
		proxy_type := s.config.Proxy.Type + 1
		settings.SetInt(libtorrent.SettingByName("proxy_type"), proxy_type)
		settings.SetInt(libtorrent.SettingByName("proxy_port"), s.config.Proxy.Port)
		settings.SetStr(libtorrent.SettingByName("proxy_hostname"), s.config.Proxy.Hostname)
		settings.SetStr(libtorrent.SettingByName("proxy_username"), s.config.Proxy.Username)
		settings.SetStr(libtorrent.SettingByName("proxy_password"), s.config.Proxy.Password)
		settings.SetBool(libtorrent.SettingByName("proxy_tracker_connections"), true)
		settings.SetBool(libtorrent.SettingByName("proxy_peer_connections"), true)
		settings.SetBool(libtorrent.SettingByName("proxy_hostnames"), true)
		settings.SetBool(libtorrent.SettingByName("force_proxy"), true)
		if proxy_type == ProxyTypeI2PSAM {
			settings.SetInt(libtorrent.SettingByName("i2p_port"), s.config.Proxy.Port)
			settings.SetStr(libtorrent.SettingByName("i2p_hostname"), s.config.Proxy.Hostname)
			settings.SetBool(libtorrent.SettingByName("allows_i2p_mixed"), false)
			settings.SetBool(libtorrent.SettingByName("allows_i2p_mixed"), true)
		}
	}

	// Set alert_mask here so it also applies on reconfigure...
	settings.SetInt(libtorrent.SettingByName("alert_mask"), int(
		libtorrent.AlertStatusNotification |
		libtorrent.AlertStorageNotification |
		libtorrent.AlertErrorNotification))

	s.packSettings = settings
	s.Session.GetHandle().ApplySettings(s.packSettings)
}

func (s *BTService) WriteState(f io.Writer) error {
	entry := libtorrent.NewEntry()
	defer libtorrent.DeleteEntry(entry)
	s.Session.GetHandle().SaveState(entry, 0xFFFF)
	_, err := f.Write([]byte(libtorrent.Bencode(entry)))
	return err
}

func (s *BTService) LoadState(f io.Reader) error {
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	entry := libtorrent.NewEntry()
	defer libtorrent.DeleteEntry(entry)
	libtorrent.Bdecode(string(data), entry)
	s.Session.GetHandle().LoadState(entry)
	return nil
}

func (s *BTService) startServices() {
	var listenPorts []string
	for p := s.config.LowerListenPort; p <= s.config.UpperListenPort; p++ {
		listenPorts = append(listenPorts, strconv.Itoa(p))
	}
	rand.Seed(time.Now().UTC().UnixNano())
	listenInterfaces := "0.0.0.0:" + listenPorts[rand.Intn(len(listenPorts))]
	if len(listenPorts) > 1 {
		listenInterfaces += ",0.0.0.0:" + listenPorts[rand.Intn(len(listenPorts))]
	}
	s.packSettings.SetStr(libtorrent.SettingByName("listen_interfaces"), listenInterfaces)

	s.log.Info("Starting LSD...")
	s.packSettings.SetBool(libtorrent.SettingByName("enable_lsd"), true)

	if s.config.DisableDHT == false {
		s.log.Info("Starting DHT...")
		bootstrap_nodes := strings.Join(dhtBootstrapNodes, ":6881,") + ":6881"
		s.packSettings.SetStr(libtorrent.SettingByName("dht_bootstrap_nodes"), bootstrap_nodes)
		s.packSettings.SetBool(libtorrent.SettingByName("enable_dht"), true)
	}

	if s.config.DisableUPNP == false {
		s.log.Info("Starting UPNP...")
		s.packSettings.SetBool(libtorrent.SettingByName("enable_upnp"), true)

		s.log.Info("Starting NATPMP...")
		s.packSettings.SetBool(libtorrent.SettingByName("enable_natpmp"), true)
	}

	s.Session.GetHandle().ApplySettings(s.packSettings)
}

func (s *BTService) stopServices() {
	if s.dialogProgressBG != nil {
		s.dialogProgressBG.Close()
	}
	s.dialogProgressBG = nil
	xbmc.ResetRPC()

	s.log.Info("Stopping LSD...")
	s.packSettings.SetBool(libtorrent.SettingByName("enable_lsd"), false)

	if s.config.DisableDHT == false {
		s.log.Info("Stopping DHT...")
		s.packSettings.SetBool(libtorrent.SettingByName("enable_dht"), false)
	}

	if s.config.DisableUPNP == false {
		s.log.Info("Stopping UPNP...")
		s.packSettings.SetBool(libtorrent.SettingByName("enable_upnp"), false)

		s.log.Info("Stopping NATPMP...")
		s.packSettings.SetBool(libtorrent.SettingByName("enable_natpmp"), false)
	}

	s.Session.GetHandle().ApplySettings(s.packSettings)
}

func (s *BTService) checkAvailableSpace(torrentHandle libtorrent.TorrentHandle) {
	var diskStatus *diskusage.DiskStatus
	if dStatus, err := diskusage.DiskUsage(config.Get().DownloadPath); err != nil {
		s.log.Warningf("Unable to retrieve the free space for %s, continuing anyway...", config.Get().DownloadPath)
		return
	} else {
		diskStatus = dStatus
	}

	if diskStatus != nil {
		torrentInfo := torrentHandle.TorrentFile()

		if torrentInfo == nil || torrentInfo.Swigcptr() == 0 {
			s.log.Warning("Missing torrent info to check available space.")
			return
		}

		status := torrentHandle.Status(uint(libtorrent.TorrentHandleQueryAccurateDownloadCounters) | uint(libtorrent.TorrentHandleQuerySavePath))
		sizeLeft := torrentInfo.TotalSize() - status.GetTotalDone()
		availableSpace := diskStatus.Free
		path := status.GetSavePath()

		s.log.Infof("Checking for sufficient space on %s...", path)
		s.log.Infof("Total size of download: %s", humanize.Bytes(uint64(torrentInfo.TotalSize())))
		s.log.Infof("All time download: %s", humanize.Bytes(uint64(status.GetAllTimeDownload())))
		s.log.Infof("Size total done: %s", humanize.Bytes(uint64(status.GetTotalDone())))
		s.log.Infof("Size left to download: %s", humanize.Bytes(uint64(sizeLeft)))
		s.log.Infof("Available space: %s", humanize.Bytes(uint64(availableSpace)))

		if availableSpace < sizeLeft {
			s.log.Errorf("Unsufficient free space on %s. Has %d, needs %d.", path, diskStatus.Free, sizeLeft)
			xbmc.Notify("Quasar", "LOCALIZE[30207]", config.AddonIcon())

			s.log.Infof("Pausing torrent %s", torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName)).GetName())
			torrentHandle.AutoManaged(false)
			torrentHandle.Pause(1)
		}
	}
}

func (s *BTService) onStateChanged(stateAlert libtorrent.StateChangedAlert) {
	switch stateAlert.GetState() {
	case libtorrent.TorrentStatusDownloading:
		torrentHandle := stateAlert.GetHandle()
		s.checkAvailableSpace(torrentHandle)
	}
}

func (s *BTService) saveResumeDataLoop() {
	saveResumeWait := time.NewTicker(time.Duration(s.config.SessionSave) * time.Second)
	defer saveResumeWait.Stop()

	for {
		select {
		case <-saveResumeWait.C:
			torrentsVector := s.Session.GetHandle().GetTorrents()
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
			switch alert.Type {
			case libtorrent.MetadataReceivedAlertAlertType:
				metadataAlert := libtorrent.SwigcptrMetadataReceivedAlert(alert.Pointer)
				torrentHandle := metadataAlert.GetHandle()
				torrentStatus := torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName))
				shaHash := torrentStatus.GetInfoHash().ToString()
				infoHash := hex.EncodeToString([]byte(shaHash))
				torrentFileName := filepath.Join(s.config.TorrentsPath, fmt.Sprintf("%s.torrent", infoHash))

				// Save .torrent
				s.log.Infof("Saving %s...", torrentFileName)
				torrentInfo := torrentHandle.TorrentFile()
				torrentFile := libtorrent.NewCreateTorrent(torrentInfo)
				defer libtorrent.DeleteCreateTorrent(torrentFile)
				torrentContent := torrentFile.Generate()
				bEncodedTorrent := []byte(libtorrent.Bencode(torrentContent))
				ioutil.WriteFile(torrentFileName, bEncodedTorrent, 0644)

			case libtorrent.StateChangedAlertAlertType:
				stateAlert := libtorrent.SwigcptrStateChangedAlert(alert.Pointer)
				s.onStateChanged(stateAlert)

			case libtorrent.SaveResumeDataAlertAlertType:
				saveResumeData := libtorrent.SwigcptrSaveResumeDataAlert(alert.Pointer)
				if saveResumeData.Swigcptr() == 0 {
					break
				}
				torrentHandle := saveResumeData.GetHandle()
				torrentStatus := torrentHandle.Status(uint(libtorrent.TorrentHandleQuerySavePath) | uint(libtorrent.TorrentHandleQueryName))
				shaHash := torrentStatus.GetInfoHash().ToString()
				infoHash := hex.EncodeToString([]byte(shaHash))
				if saveResumeData.Swigcptr() == 0 {
					break
				}
				entry := saveResumeData.ResumeData()
				bEncoded := []byte(libtorrent.Bencode(entry))

				// s.log.Infof("Saving resume data for %s to %s.fastresume", torrentStatus.GetName(), infoHash)
				path := filepath.Join(s.config.TorrentsPath, fmt.Sprintf("%s.fastresume", infoHash))
				ioutil.WriteFile(path, bEncoded, 0644)
			}
		}
	}
}

func (s *BTService) loadTorrentFiles() {
	pattern := filepath.Join(s.config.TorrentsPath, "*.torrent")
	files, _ := filepath.Glob(pattern)
	for _, torrentFile := range files {
		torrentParams := libtorrent.NewAddTorrentParams()
		defer libtorrent.DeleteAddTorrentParams(torrentParams)

		s.log.Infof("Loading torrent file %s", torrentFile)

		info := libtorrent.NewTorrentInfo(torrentFile)
		defer libtorrent.DeleteTorrentInfo(info)
		torrentParams.SetTorrentInfo(info)
		torrentParams.SetSavePath(s.config.DownloadPath)

		fastResumeFile := strings.Replace(torrentFile, ".torrent", ".fastresume", 1)

		if _, err := os.Stat(fastResumeFile); err == nil {
			fastResumeData, err := ioutil.ReadFile(fastResumeFile)
			if err != nil {
				s.log.Errorf("Error reading fastresume file: %s", err.Error())
				os.Remove(fastResumeFile)
			} else {
				fastResumeVector := libtorrent.NewStdVectorChar()
				defer libtorrent.DeleteStdVectorChar(fastResumeVector)
				for _, c := range fastResumeData {
					fastResumeVector.Add(c)
				}
				torrentParams.SetResumeData(fastResumeVector)
			}
		}

		torrentHandle := s.Session.GetHandle().AddTorrent(torrentParams)

		if torrentHandle == nil {
			s.log.Errorf("Error adding torrent file for %s", torrentFile)
			if _, err := os.Stat(torrentFile); err == nil {
				os.Remove(torrentFile)
			}
			if _, err := os.Stat(fastResumeFile); err == nil {
				os.Remove(fastResumeFile)
			}
			continue
		}
	}
}

func (s *BTService) downloadProgress() {
	rotateTicker := time.NewTicker(5 * time.Second)
	defer rotateTicker.Stop()

	showNext := 0
	for {
		select {
		case <-rotateTicker.C:
			if s.Session.GetHandle().IsPaused() && s.dialogProgressBG != nil {
				s.dialogProgressBG.Close()
				s.dialogProgressBG = nil
				continue
			}

			torrentsVector := s.Session.GetHandle().GetTorrents()
			torrentsVectorSize := int(torrentsVector.Size())
			totalProgress := 0
			activeTorrents := make([]*activeTorrent, 0)

			for i := 0; i < torrentsVectorSize; i++ {
				torrentHandle := torrentsVector.Get(i)
				if torrentHandle.IsValid() == false {
					continue
				}

				torrentStatus := torrentHandle.Status(uint(libtorrent.TorrentHandleQueryName))
				if torrentStatus.GetHasMetadata() == false  || torrentStatus.GetPaused() || s.Session.GetHandle().IsPaused() {
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
				} else {
					finished_time := torrentStatus.GetFinishedTime()
					if s.config.SeedTimeLimit > 0 {
						if finished_time >= s.config.SeedTimeLimit {
							s.log.Noticef("Seeding time limit reached, pausing %s", torrentName)
							torrentHandle.AutoManaged(false)
							torrentHandle.Pause(1)
							continue
						}
					}
					if s.config.SeedTimeRatioLimit > 0 {
						timeRatio := 0
						download_time := torrentStatus.GetActiveTime() - finished_time
						if download_time > 1 {
							timeRatio = finished_time * 100 / download_time
						}
						if timeRatio >= s.config.SeedTimeRatioLimit {
							s.log.Noticef("Seeding time ratio reached, pausing %s", torrentName)
							torrentHandle.AutoManaged(false)
							torrentHandle.Pause(1)
							continue
						}
					}
					if s.config.ShareRatioLimit > 0 {
						ratio := int64(0)
						allTimeDownload := torrentStatus.GetAllTimeDownload()
						if allTimeDownload > 0 {
							ratio = torrentStatus.GetAllTimeUpload() * 100 / allTimeDownload
						}
						if ratio >= int64(s.config.ShareRatioLimit) {
							s.log.Noticef("Share ratio reached, pausing %s", torrentName)
							torrentHandle.AutoManaged(false)
							torrentHandle.Pause(1)
						}
					}
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
					showNext += 1
				}
				if s.dialogProgressBG == nil {
					s.dialogProgressBG = xbmc.NewDialogProgressBG("Quasar", "")
				}
				s.dialogProgressBG.Update(showProgress, "Quasar", showTorrent)
			} else if s.dialogProgressBG != nil {
				s.dialogProgressBG.Close()
				s.dialogProgressBG = nil
			}
		}
	}
}

func (s *BTService) alertsConsumer() {
	defer s.alertsBroadcaster.Close()

	ltOneSecond := libtorrent.Seconds(libtorrentAlertWaitTime)
	s.log.Info("Consuming alerts...")
	for {
		select {
		case <-s.closing:
			s.log.Info("Closing all alert channels...")
			return
		default:
			if s.Session.GetHandle().WaitForAlert(ltOneSecond).Swigcptr() == 0 {
				continue
			}
			var alerts libtorrent.StdVectorAlerts
			alerts = s.Session.GetHandle().PopAlerts()
			queueSize := alerts.Size()
			for i := 0; i < int(queueSize); i++ {
				ltAlert := alerts.Get(i)
				alert := &Alert{
					Type: ltAlert.Type(),
					Category: ltAlert.Category(),
					What: ltAlert.What(),
					Message: ltAlert.Message(),
					Pointer: ltAlert.Swigcptr(),
				}
				s.alertsBroadcaster.Broadcast(alert)
			}
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
		if alert.Category & int(libtorrent.AlertErrorNotification) != 0 {
			s.libtorrentLog.Errorf("%s: %s", alert.What, alert.Message)
		} else if alert.Category & int(libtorrent.AlertDebugNotification) != 0 {
			s.libtorrentLog.Debugf("%s: %s", alert.What, alert.Message)
		} else if alert.Category & int(libtorrent.AlertPerformanceWarning) != 0 {
			s.libtorrentLog.Warningf("%s: %s", alert.What, alert.Message)
		} else {
			s.libtorrentLog.Noticef("%s: %s", alert.What, alert.Message)
		}
	}
}
