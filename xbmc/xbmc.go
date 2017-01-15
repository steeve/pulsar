package xbmc

func UpdateAddonRepos() (retVal string) {
	executeJSONRPCEx("UpdateAddonRepos", &retVal, nil)
	return
}

func ResetRPC() (retVal string) {
	executeJSONRPCEx("Reset", &retVal, nil)
	return
}

func Refresh() (retVal string) {
	executeJSONRPCEx("Refresh", &retVal, nil)
	return
}

func VideoLibraryScan() {
	var retVal string
	executeJSONRPC("VideoLibrary.Scan", &retVal, nil)
}

func VideoLibraryClean() {
	var retVal string
	executeJSONRPC("VideoLibrary.Clean", &retVal, nil)
}

func VideoLibraryGetMovies() (movies *VideoLibraryMovies) {
	params := map[string]interface{}{"properties": []interface{}{"imdbnumber"}}
	ret := executeJSONRPCO("VideoLibrary.GetMovies", &movies, params)
	if ret != nil {
		log.Error(ret)
	}
	return movies
}

func VideoLibraryGetShows() (shows *VideoLibraryShows) {
	params := map[string]interface{}{"properties": []interface{}{"imdbnumber"}}
	ret := executeJSONRPCO("VideoLibrary.GetTVShows", &shows, params)
	if ret != nil {
		log.Error(ret)
	}
	return shows
}

func TranslatePath(path string) (retVal string) {
	executeJSONRPCEx("TranslatePath", &retVal, Args{path})
	return
}

func PlayURL(url string) {
	var item struct {
		File string `json:"file"`
	}
	item.File = url
	retVal := 0
	executeJSONRPC("Player.Open", &retVal, Args{item})
}

const (
	ISO_639_1 = iota
	ISO_639_2
	EnglishName
)

func ConvertLanguage(language string, format int) string {
	retVal := ""
	executeJSONRPCEx("ConvertLanguage", &retVal, Args{language, format})
	return retVal
}

func GetLanguage(format int) string {
	retVal := ""
	executeJSONRPCEx("GetLanguage", &retVal, Args{format})
	return retVal
}

func GetLanguageISO_639_1() string{
	language := GetLanguage(ISO_639_1)
	if language == "" {
		switch GetLanguage(EnglishName) {
		case "Chinese (Simple)":      return "zh"
		case "Chinese (Traditional)": return "zh"
		case "English (Australia)":   return "en"
		case "English (New Zealand)": return "en"
		case "English (US)":          return "en"
		case "French (Canada)":       return "fr"
		case "Hindi (Devanagiri)":    return "hi"
		case "Mongolian (Mongolia)":  return "mn"
		case "Persian (Iran)":        return "fa"
		case "Portuguese (Brazil)":   return "pt"
		case "Serbian (Cyrillic)":    return "sr"
		case "Spanish (Argentina)":   return "es"
		case "Spanish (Mexico)":      return "es"
		case "Tamil (India)":         return "ta"
		default:                      return "en"
		}
	}
	return language
}
