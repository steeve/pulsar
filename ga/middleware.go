package ga

import "github.com/gin-gonic/gin"

func GATracker() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		path := ctx.Request.URL.Path
		query := ctx.Request.URL.RawQuery
		if query != "" {
			path += "?" + query
		}
		go TrackPageView(path)
		ctx.Next()
	}
}
