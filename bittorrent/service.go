package bittorrent

import (
	"runtime"
	"sync"

	"github.com/op/go-logging"
	"github.com/steeve/libtorrent-go"
)

const (
	libtorrentAlertWaitTime = 1
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
	Session       libtorrent.Session
	config        *BTConfiguration
	log           *logging.Logger
	libtorrentLog *logging.Logger
	alertsMutex   sync.RWMutex
	alertHandlers []AlertHandler
}

type AlertHandler func(alert libtorrent.Alert)

func NewBTService() *BTService {
	s := &BTService{
		Session:       libtorrent.NewSession(),
		log:           logging.MustGetLogger("btservice"),
		libtorrentLog: logging.MustGetLogger("libtorrent"),
		alertHandlers: make([]AlertHandler, 0),
	}
	// Ensure we properly free the session object.
	runtime.SetFinalizer(s, func(s *BTService) {
		libtorrent.DeleteSession(s.Session)
	})
	go s.consumeAlerts()

	return s
}

func (s *BTService) Start() {
	s.log.Info("Starting streamer BTService...")
	s.startServices()
}

func (s *BTService) Stop() {
	s.log.Info("Stopping BTServices...")
	s.stopServices()
}

func (s *BTService) Configure(c *BTConfiguration) {
	s.config = c
	settings := s.Session.Settings()

	s.log.Info("Setting Session settings...")

	settings.SetUser_agent("")

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
	}

	settings.SetTorrent_connect_boost(100)
	settings.SetRate_limit_ip_overhead(true)

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

	s.Session.Set_settings(settings)

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

func (s *BTService) startServices() {
	errCode := libtorrent.NewError_code()
	defer libtorrent.DeleteError_code(errCode)
	ports := libtorrent.NewStd_pair_int_int(s.config.LowerListenPort, s.config.UpperListenPort)
	defer libtorrent.DeleteStd_pair_int_int(ports)
	s.Session.Listen_on(ports, errCode)

	s.log.Info("Starting DHT...")
	s.Session.Start_dht()
	for _, node := range dhtBootstrapNodes {
		pair := libtorrent.NewStd_pair_string_int(node, 6881)
		defer libtorrent.DeleteStd_pair_string_int(pair)
		s.Session.Add_dht_router(pair)
	}

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

func (s *BTService) consumeAlerts() {
	// s.Session.Set_alert_mask(uint(libtorrent.AlertAll_categories))
	s.Session.Set_alert_mask(uint(libtorrent.AlertError_notification |
		libtorrent.AlertStatus_notification |
		libtorrent.AlertStorage_notification))
	for {
		s.Session.Wait_for_alert(libtorrent.Seconds(libtorrentAlertWaitTime))
		alert := s.Session.Pop_alert()
		if alert.Swigcptr() == 0 {
			continue
		}
		alertCategory := alert.Category()
		if alertCategory&int(libtorrent.AlertError_notification) != 0 {
			// s.libtorrentLog.Error("%s: %s", alert.What(), alert.Message())
		} else if alertCategory&int(libtorrent.AlertDebug_notification) != 0 {
			s.libtorrentLog.Debug("%s: %s", alert.What(), alert.Message())
		} else if alertCategory&int(libtorrent.AlertPerformance_warning) != 0 {
			s.libtorrentLog.Warning("%s: %s", alert.What(), alert.Message())
		} else {
			s.libtorrentLog.Notice("%s: %s", alert.What(), alert.Message())
		}
		for _, handler := range s.alertHandlers {
			if handler != nil {
				go handler(alert)
			}
		}
	}
}

func (s *BTService) AlertsBind(handler AlertHandler) {
	s.alertsMutex.Lock()
	s.alertHandlers = append(s.alertHandlers, handler)
	s.alertsMutex.Unlock()
}

func (s *BTService) AlertsUnbind(handler AlertHandler) {
	s.alertsMutex.Lock()
	// idx := -1
	// for i, h := range s.alertHandlers {
	// 	if h == handler {
	// 		idx = i
	// 		break
	// 	}
	// }
	// if idx >= 0 {
	// 	s.alertHandlers = append(s.alertHandlers[:idx], s.alertHandlers[idx+1]...)
	// }
	s.alertsMutex.Unlock()
}
