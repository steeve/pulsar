package xbmc

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
