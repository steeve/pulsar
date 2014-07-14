package bittorrent

import (
	"runtime"
	"sync"

	"github.com/op/go-logging"
	"github.com/steeve/libtorrent-go"
)

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
	Config        BTConfiguration
	log           *logging.Logger
	libtorrentLog *logging.Logger
	alertsMutex   sync.RWMutex
	alertHandlers []AlertHandler
}

type AlertHandler func(alert libtorrent.Alert)

func NewBTService(config BTConfiguration) *BTService {
	s := &BTService{
		Session:       libtorrent.NewSession(),
		Config:        config,
		log:           logging.MustGetLogger("BTService"),
		libtorrentLog: logging.MustGetLogger("libtorrent"),
		alertHandlers: make([]AlertHandler, 0),
	}
	// Ensure we properly free the session object.
	runtime.SetFinalizer(s, func(s *BTService) {
		libtorrent.DeleteSession(s.Session)
	})
	go s.consumeAlerts()
	s.configureSession()

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

func (s *BTService) configureSession() {
	settings := s.Session.Settings()

	s.log.Info("Setting Session settings...")

	settings.SetUser_agent("")

	settings.SetRequest_timeout(5)
	settings.SetPeer_connect_timeout(2)
	settings.SetAnnounce_to_all_trackers(true)
	settings.SetAnnounce_to_all_tiers(true)
	settings.SetConnection_speed(100)
	if s.Config.MaxDownloadRate > 0 {
		settings.SetDownload_rate_limit(s.Config.MaxDownloadRate * 1024)
	}
	if s.Config.MaxUploadRate > 0 {
		settings.SetUpload_rate_limit(s.Config.MaxUploadRate * 1024)
	}

	settings.SetTorrent_connect_boost(100)
	settings.SetRate_limit_ip_overhead(true)

	s.Session.Set_settings(settings)

	s.log.Info("Setting Encryption settings...")
	encryptionSettings := libtorrent.NewPe_settings()
	defer libtorrent.DeletePe_settings(encryptionSettings)
	encryptionSettings.SetOut_enc_policy(byte(libtorrent.Pe_settingsForced))
	encryptionSettings.SetIn_enc_policy(byte(libtorrent.Pe_settingsForced))
	encryptionSettings.SetAllowed_enc_level(byte(libtorrent.Pe_settingsBoth))
	encryptionSettings.SetPrefer_rc4(true)
	s.Session.Set_pe_settings(encryptionSettings)

	if s.Config.Proxy != nil {
		s.log.Info("Setting Proxy settings...")
		proxy := libtorrent.NewProxy_settings()
		defer libtorrent.DeleteProxy_settings(proxy)
		proxy.SetHostname(s.Config.Proxy.Hostname)
		proxy.SetPort(uint16(s.Config.Proxy.Port))
		proxy.SetUsername(s.Config.Proxy.Username)
		proxy.SetPassword(s.Config.Proxy.Password)
		proxy.SetXtype(byte(s.Config.Proxy.Type))
		proxy.SetProxy_hostnames(true)
		proxy.SetProxy_peer_connections(true)
		s.Session.Set_proxy(proxy)
	}
}

func (s *BTService) startServices() {
	errCode := libtorrent.NewError_code()
	defer libtorrent.DeleteError_code(errCode)
	ports := libtorrent.NewStd_pair_int_int(s.Config.LowerListenPort, s.Config.UpperListenPort)
	defer libtorrent.DeleteStd_pair_int_int(ports)
	s.Session.Listen_on(ports, errCode)

	s.log.Info("Starting DHT...")
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

func (s *BTService) consumeAlerts() {
	// s.Session.Set_alert_mask(uint(libtorrent.AlertAll_categories))
	s.Session.Set_alert_mask(uint(libtorrent.AlertError_notification |
		libtorrent.AlertStatus_notification |
		libtorrent.AlertStorage_notification))
	for {
		s.Session.Wait_for_alert(libtorrent.Seconds(30))
		alert := s.Session.Pop_alert()
		if alert.Swigcptr() == 0 {
			continue
		}
		switch libtorrent.LibtorrentAlertCategory_t(alert.Category()) {
		case libtorrent.AlertError_notification:
			s.libtorrentLog.Error("%s: %s", alert.What(), alert.Message())
			break
		default:
			s.libtorrentLog.Info("%s: %s", alert.What(), alert.Message())
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
