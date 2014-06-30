package streamer

type ProxySettings struct {
	Hostname             string
	Port                 int
	Username             string
	Password             string
	Type                 int
	Hostnames            bool
	ProxyPeerConnections bool
}

type StreamerConfiguration struct {
	MaxDownloadRate int
	MaxUploadRate   int
	LowerListenPort int
	UpperListenPort int
	DownloadPath    string
	Proxy           *ProxySettings
}
