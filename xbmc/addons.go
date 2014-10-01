package xbmc

import "encoding/xml"

type AddonsList struct {
	Addons []*struct {
		ID   string `json:"addonid"`
		Type string `json:"type"`
	} `json:"addons"`
}

func GetAddons(addonType string) *AddonsList {
	addons := AddonsList{}
	executeJSONRPC("Addons.GetAddons", &addons, Args{addonType})
	return &addons
}

func ExecuteAddon(addonId string, args ...interface{}) {
	var retVal string
	executeJSONRPC("Addons.ExecuteAddon", &retVal, Args{addonId, args})
}

type Addon struct {
	XMLName      xml.Name `xml:"addon"`
	Id           string   `xml:"id,attr"`
	Name         string   `xml:"name,attr"`
	Version      string   `xml:"version,attr"`
	ProviderName string   `xml:"provider-name,attr"`

	Requires []struct {
		Addon    string `xml:"addon,attr"`
		Version  string `xml:"version,attr"`
		Optional string `xml:"optional,attr,omitempty"`
	} `xml:"requires>import,omitempty"`

	Extensions []struct {
		Point string `xml:"point,attr"`

		// xbmc.python.pluginsource
		// xbmc.service
		Library string `xml:"library,attr,omitempty"`

		// xbmc.python.pluginsource
		Provides string `xml:"provides,omitempty"`

		// xbmc.service
		Start string `xml:"start,attr,omitempty"`

		// xbmc.addon.metadata
		Language  string `xml:"language,omitempty"`
		Platform  string `xml:"platform,omitempty"`
		License   string `xml:"license,omitempty"`
		Forum     string `xml:"forum,omitempty"`
		Website   string `xml:"website,omitempty"`
		Email     string `xml:"email,omitempty"`
		Source    string `xml:"source,omitempty"`
		Broken    string `xml:"broken,omitempty"`
		Summaries []struct {
			Text string `xml:",chardata"`
			Lang string `xml:"lang,attr"`
		} `xml:"summary,omitempty"`
		Disclaimers []struct {
			Text string `xml:",chardata"`
			Lang string `xml:"lang,attr"`
		} `xml:"disclaimer,omitempty"`
		Descriptions []struct {
			Text string `xml:",chardata"`
			Lang string `xml:"lang,attr"`
		} `xml:"description,omitempty"`

		// xbmc.addon.repository
		Name string `xml:"name,attr,omitempty"`
		Info *struct {
			Text       string `xml:",chardata"`
			Compressed bool   `xml:"compressed,attr"`
		} `xml:"info,omitempty"`
		Checksum string `xml:"checksum,omitempty"`
		Datadir  *struct {
			Text string `xml:",chardata"`
			Zip  bool   `xml:"zip,attr"`
		} `xml:"datadir,omitempty"`

		// xbmc.gui.skin
		DefaultResolution     string `xml:"defaultresolution,omitempty"`
		DefaultResolutionWide string `xml:"defaultresolutionwide,omitempty"`
		DefaultThemeName      string `xml:"defaultthemename,omitempty"`
		EffectsSlowdown       string `xml:"effectslowdown,omitempty"`
		Debugging             string `xml:"debugging,omitempty"`
		Resolutions           []struct {
			Width   int    `xml:"width,attr"`
			Height  int    `xml:"height,attr"`
			Aspect  string `xml:"aspect,attr"`
			Default bool   `xml:"default,attr"`
			Folder  string `xml:"folder,attr"`
		} `xml:"res,omitempty"`
	} `xml:"extension"`
}

type AddonList struct {
	XMLName xml.Name `xml:"addons"`
	Addons  []Addon
}
