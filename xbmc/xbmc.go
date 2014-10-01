package xbmc

func TranslatePath(path string) (retVal string) {
	executeJSONRPCEx("TranslatePath", &retVal, Args{path})
	return
}
