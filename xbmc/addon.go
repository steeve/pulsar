package xbmc

// <?xml version="1.0" encoding="UTF-8" standalone="yes"?>
// <addon id="plugin.video.pulsar" name="Pulsar" version="0.0.1" provider-name="steeve">
//     <requires>
//         <import addon="xbmc.python" version="2.1.0"/>
//     </requires>
//     <extension point="xbmc.python.pluginsource" library="main.py">
//         <provides>video</provides>
//     </extension>
//     <extension point="xbmc.service" library="jsonrpc_service.py" start="login"/>
// <!--     <extension point="xbmc.service" library="service.py" start="login"/>
//  -->    <extension point="xbmc.addon.metadata">
//         <platform>all</platform>
//         <language></language>
//     </extension>
// </addon>

type Import struct {
	Addon    string
	Version  string
	Optional bool
}

type Extension struct {
	Point string
}

type PulsarExtension struct {
	Extension
	CmdLine string
}

type Addon struct {
	ID           string
	Name         string
	Version      string
	ProviderName string
	Requires     []*Import
	Extensions   []*Extension
}
