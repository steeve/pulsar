package bittorrent

import (
	"net"
	"runtime"
	"time"

	"github.com/op/go-logging"
	"github.com/steeve/libtorrent-go"
	"github.com/steeve/pulsar/broadcast"
	"github.com/steeve/pulsar/util"
)

const (
	libtorrentAlertWaitTime = 1 // 1 second
	internetCheckAddress    = "google.com"
)

var dhtBootstrapNodes = []string{
	"router.bittorrent.com",
	"router.utorrent.com",
	"dht.transmissionbt.com",
	"dht.aelitis.com", // Vuze
}

type ProxySettings struct {
	Hostname             string
	Port                 int
	Username             string
	Password             string
	Type                 int
	Hostnames            bool
	ProxyPeerConnections bool
}

type BTConfiguration struct {
	MaxDownloadRate int
	MaxUploadRate   int
	LowerListenPort int
	UpperListenPort int
	DownloadPath    string
	Proxy           *ProxySettings
}

type BTService struct {
	Session           libtorrent.Session
	config            *BTConfiguration
	log               *logging.Logger
	libtorrentLog     *logging.Logger
	alertsBroadcaster *broadcast.Broadcaster
	closing           chan interface{}
}

func NewBTService() *BTService {
	s := &BTService{
		Session:           libtorrent.NewSession(),
		log:               logging.MustGetLogger("btservice"),
		libtorrentLog:     logging.MustGetLogger("libtorrent"),
		alertsBroadcaster: broadcast.NewBroadcaster(),
		closing:           make(chan interface{}),
	}
	// Ensure we properly free the session object.
	runtime.SetFinalizer(s, func(s *BTService) {
		s.Close()
	})
	go s.alertsConsumer()
	go s.logAlerts()
	go s.internetMonitor()

	return s
}

func (s *BTService) internetMonitor() {
	lastConnected := false
	oneSecond := time.Tick(1 * time.Second)
	for {
		select {
		case <-s.closing:
			return
		case <-oneSecond:
			_, err := net.LookupHost(internetCheckAddress)
			connected := (err == nil)
			if connected != lastConnected {
				lastConnected = connected
				if connected {
					s.log.Info("Internet connection status changed, listening...")
					s.Listen()
					s.startServices()
				} else {
					s.log.Info("No internet connection available")
					s.stopServices()
				}
			}
		}
	}
}

func (s *BTService) Close() {
	close(s.closing)
	libtorrent.DeleteSession(s.Session)
}

func (s *BTService) Start() {
	s.log.Info("Starting streamer BTService...")
}

func (s *BTService) Stop() {
	s.log.Info("Stopping BTServices...")
	s.stopServices()
}

func (s *BTService) Configure(c *BTConfiguration) {
	s.config = c
	settings := s.Session.Settings()

	s.log.Info("Setting Session settings...")

	settings.SetUser_agent(util.UserAgent())

	settings.SetRequest_timeout(5)
	settings.SetPeer_connect_timeout(2)
	settings.SetAnnounce_to_all_trackers(true)
	settings.SetAnnounce_to_all_tiers(true)
	settings.SetConnection_speed(100)
	if s.config.MaxDownloadRate > 0 {
		settings.SetDownload_rate_limit(s.config.MaxDownloadRate * 1024)
	}
	if s.config.MaxUploadRate > 0 {
		settings.SetUpload_rate_limit(s.config.MaxUploadRate * 1024)
		// If we have an upload rate, use the nicer bittyrant choker
		settings.SetChoking_algorithm(int(libtorrent.Session_settingsBittyrant_choker))
	}

	settings.SetTorrent_connect_boost(100)
	settings.SetRate_limit_ip_overhead(true)
	settings.SetNo_atime_storage(true)
	settings.SetAnnounce_double_nat(true)
	settings.SetPrioritize_partial_pieces(true)
	settings.SetIgnore_limits_on_local_network(true)

	// Prioritize people starting downloads
	// settings.SetSeed_choking_algorithm(int(libtorrent.Session_settingsAnti_leech))

	// copied from qBitorrent at
	// https://github.com/qbittorrent/qBittorrent/blob/master/src/qtlibtorrent/qbtsession.cpp
	settings.SetUpnp_ignore_nonrouters(true)
	settings.SetLazy_bitfields(true)
	settings.SetStop_tracker_timeout(1)
	settings.SetAuto_scrape_interval(1200)    // 20 minutes
	settings.SetAuto_scrape_min_interval(900) // 15 minutes
	settings.SetIgnore_limits_on_local_network(true)
	settings.SetRate_limit_utp(false)
	settings.SetMixed_mode_algorithm(int(libtorrent.Session_settingsPeer_proportional))

	setPlatformSpecificSettings(settings)

	s.Session.Set_settings(settings)

	// Add all the libtorrent extensions
	s.Session.Add_extensions()

	s.log.Info("Setting Encryption settings...")
	encryptionSettings := libtorrent.NewPe_settings()
	defer libtorrent.DeletePe_settings(encryptionSettings)
	encryptionSettings.SetOut_enc_policy(byte(libtorrent.Pe_settingsForced))
	encryptionSettings.SetIn_enc_policy(byte(libtorrent.Pe_settingsForced))
	encryptionSettings.SetAllowed_enc_level(byte(libtorrent.Pe_settingsBoth))
	encryptionSettings.SetPrefer_rc4(true)
	s.Session.Set_pe_settings(encryptionSettings)

	if s.config.Proxy != nil {
		s.log.Info("Setting Proxy settings...")
		proxy := libtorrent.NewProxy_settings()
		defer libtorrent.DeleteProxy_settings(proxy)
		proxy.SetHostname(s.config.Proxy.Hostname)
		proxy.SetPort(uint16(s.config.Proxy.Port))
		proxy.SetUsername(s.config.Proxy.Username)
		proxy.SetPassword(s.config.Proxy.Password)
		proxy.SetXtype(byte(s.config.Proxy.Type))
		proxy.SetProxy_hostnames(true)
		proxy.SetProxy_peer_connections(true)
		s.Session.Set_proxy(proxy)
	}
}

func (s *BTService) Listen() {
	errCode := libtorrent.NewError_code()
	defer libtorrent.DeleteError_code(errCode)
	ports := libtorrent.NewStd_pair_int_int(s.config.LowerListenPort, s.config.UpperListenPort)
	defer libtorrent.DeleteStd_pair_int_int(ports)
	s.Session.Listen_on(ports, errCode)
}

func (s *BTService) startServices() {
	s.log.Info("Starting DHT...")
	for _, node := range dhtBootstrapNodes {
		pair := libtorrent.NewStd_pair_string_int(node, 6881)
		defer libtorrent.DeleteStd_pair_string_int(pair)
		s.Session.Add_dht_router(pair)
	}
	s.Session.Start_dht()

	s.log.Info("Starting LSD...")
	s.Session.Start_lsd()

	s.log.Info("Starting UPNP...")
	s.Session.Start_upnp()

	s.log.Info("Starting NATPMP...")
	s.Session.Start_natpmp()
}

func (s *BTService) stopServices() {
	s.log.Info("Stopping DHT...")
	s.Session.Stop_dht()

	s.log.Info("Stopping LSD...")
	s.Session.Stop_lsd()

	s.log.Info("Stopping UPNP...")
	s.Session.Stop_upnp()

	s.log.Info("Stopping NATPMP...")
	s.Session.Stop_natpmp()
}

func (s *BTService) alertsConsumer() {
	s.Session.Set_alert_mask(uint(libtorrent.AlertError_notification |
		libtorrent.AlertStatus_notification |
		libtorrent.AlertStorage_notification))

	defer s.alertsBroadcaster.Close()

	ltOneSecond := libtorrent.Seconds(libtorrentAlertWaitTime)
	for {
		select {
		case <-s.closing:
			s.log.Info("Closing all alert channels...")
			return
		default:
			if s.Session.Wait_for_alert(ltOneSecond).Swigcptr() == 0 {
				continue
			}
			alert := &Alert{s.Session.Pop_alert()}
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
	alerts, done := s.Alerts()
	defer close(done)
	for alert := range alerts {
		alertCategory := alert.Category()
		if alertCategory&int(libtorrent.AlertError_notification) != 0 {
			s.libtorrentLog.Error("%s: %s", alert.What(), alert.Message())
		} else if alertCategory&int(libtorrent.AlertDebug_notification) != 0 {
			s.libtorrentLog.Debug("%s: %s", alert.What(), alert.Message())
		} else if alertCategory&int(libtorrent.AlertPerformance_warning) != 0 {
			s.libtorrentLog.Warning("%s: %s", alert.What(), alert.Message())
		} else {
			s.libtorrentLog.Notice("%s: %s", alert.What(), alert.Message())
		}
	}
}
