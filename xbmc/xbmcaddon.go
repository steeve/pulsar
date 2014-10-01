package xbmc

import "strconv"

type AddonInfo struct {
	Author      string `xml:"id,attr"`
	Changelog   string
	Description string
	Disclaimer  string
	Fanart      string
	Icon        string
	Id          string
	Name        string
	Path        string
	Profile     string
	Stars       string
	Summary     string
	Type        string
	Version     string
}

func GetAddonInfo() *AddonInfo {
	retVal := AddonInfo{}
	executeJSONRPCEx("GetAddonInfo", &retVal, nil)
	return &retVal
}

func GetSettingString(id string) (retVal string) {
	executeJSONRPCEx("GetSetting", &retVal, Args{id})
	return
}

func GetSettingInt(id string) int {
	val, _ := strconv.Atoi(GetSettingString(id))
	return val
}

func GetSettingBool(id string) bool {
	return GetSettingString(id) == "true"
}

func SetSetting(id string, value interface{}) {
	retVal := 0
	executeJSONRPCEx("GetSetting", &retVal, Args{id, value})
}
