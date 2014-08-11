package xbmc

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

type AddonSettings struct {
	Settings []*struct {
		ID    string `xml:"id"`
		Value string `xml:"value"`
	}
}

func GetSettings(addonId string) map[string]string {
	return nil
}
