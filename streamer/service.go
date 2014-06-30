package streamer

import (
	"log"
	"runtime"

	"github.com/steeve/libtorrent-go"
)

type Streamer struct {
	session libtorrent.Session
	config  StreamerConfiguration
}

var streamer *Streamer

func newStreamer(config StreamerConfiguration) *Streamer {
	s := &Streamer{
		session: libtorrent.NewSession(),
		config:  config,
	}
	// Ensure we properly free the session object.
	runtime.SetFinalizer(s, func(s *Streamer) {
		log.Println("Freeing session object.")
		libtorrent.DeleteSession(s.session)
	})
	s.configureSession()

	return s
}

func Start(config StreamerConfiguration) {
	if streamer == nil {
		log.Println("Starting streamer service...")
		streamer = newStreamer(config)
		streamer.startServices()
	} else {
		log.Println("Streamer service already started.")
	}
}

func (s *Streamer) Stop() {
	s.stopServices()
	streamer = nil
}

func (s *Streamer) configureSession() {
	settings := s.session.Settings()

	log.Println("Setting Session settings...")

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

	s.session.Set_settings(settings)

	log.Println("Setting Encryption settings...")
	encryptionSettings := libtorrent.NewPe_settings()
	defer libtorrent.DeletePe_settings(encryptionSettings)
	encryptionSettings.SetOut_enc_policy(byte(libtorrent.Pe_settingsForced))
	encryptionSettings.SetIn_enc_policy(byte(libtorrent.Pe_settingsForced))
	encryptionSettings.SetAllowed_enc_level(byte(libtorrent.Pe_settingsBoth))
	encryptionSettings.SetPrefer_rc4(true)
	s.session.Set_pe_settings(encryptionSettings)

	if s.config.Proxy != nil {
		proxy := libtorrent.NewProxy_settings()
		defer libtorrent.DeleteProxy_settings(proxy)
		proxy.SetHostname(s.config.Proxy.Hostname)
		proxy.SetPort(uint16(s.config.Proxy.Port))
		proxy.SetUsername(s.config.Proxy.Username)
		proxy.SetPassword(s.config.Proxy.Password)
		proxy.SetXtype(byte(s.config.Proxy.Type))
		proxy.SetProxy_hostnames(true)
		proxy.SetProxy_peer_connections(true)
		s.session.Set_proxy(proxy)
	}
}

func (s *Streamer) startServices() {
	errCode := libtorrent.NewError_code()
	defer libtorrent.DeleteError_code(errCode)
	ports := libtorrent.NewStd_pair_int_int(s.config.LowerListenPort, s.config.UpperListenPort)
	defer libtorrent.DeleteStd_pair_int_int(ports)
	s.session.Listen_on(ports, errCode)

	log.Println("Starting DHT...")
	s.session.Start_dht()

	log.Println("Starting LSD...")
	s.session.Start_lsd()

	log.Println("Starting UPNP...")
	s.session.Start_upnp()

	log.Println("Starting NATPMP...")
	s.session.Start_natpmp()
}

func (s *Streamer) stopServices() {
	log.Println("Stopping DHT...")
	s.session.Stop_dht()

	log.Println("Stopping LSD...")
	s.session.Stop_lsd()

	log.Println("Stopping UPNP...")
	s.session.Stop_upnp()

	log.Println("Stopping NATPMP...")
	s.session.Stop_natpmp()
}
