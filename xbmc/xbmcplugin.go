package xbmc

func SetResolvedUrl(url string) {
	retVal := -1
	executeJSONRPCEx("SetResolvedUrl", &retVal, Args{url})
}
