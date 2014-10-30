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
