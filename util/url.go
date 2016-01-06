package util

// import (
// 	"net/http"
// 	"net/url"

// 	"github.com/i96751414/pulsar/providers"
// )

// func MakeProviderURL(p providers.TorrentProvider, path string) string {
// 	return MakeProviderURLWithQuery(p, path, nil)
// }

// func MakeProviderURLWithQuery(p providers.TorrentProvider, path string, query map[string]string) string {
// 	urlObj, _ := url.Parse(p.URL())
// 	pathObj, _ := url.Parse(path)
// 	urlObj = urlObj.ResolveReference(pathObj)
// 	v := url.Values{}
// 	for key, value := range query {
// 		v.Set(key, value)
// 	}
// 	urlObj.RawQuery = v.Encode()
// 	return urlObj.String()
// }

// func httpGet(url string) (resp *http.Response, err error) {
// 	return http.Get(url)
// }
