package api

import (
	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/youtube"
)

func PlayYoutubeVideo(ctx *gin.Context) {
	youtubeId := ctx.Params.ByName("id")
	streams, err := youtube.Resolve(youtubeId)
	if err != nil {
		ctx.AbortWithError(404, err)
	}
	for _, stream := range streams {
		ctx.Redirect(302, stream)
		return
	}
}
