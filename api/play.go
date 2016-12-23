package api

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/bittorrent"
	"github.com/scakemyer/quasar/util"
	"github.com/scakemyer/quasar/xbmc"
)

func Play(btService *bittorrent.BTService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		uri := ctx.Request.URL.Query().Get("uri")
		index := ctx.Request.URL.Query().Get("index")
		resume := ctx.Request.URL.Query().Get("resume")
		contentType := ctx.Request.URL.Query().Get("type")
		tmdb := ctx.Request.URL.Query().Get("tmdb")
		runtime := ctx.Request.URL.Query().Get("runtime")

		if uri == "" && resume == "" {
			return
		}

		fileIndex := -1
		if index != "" {
			fIndex, err := strconv.Atoi(index)
			if err == nil {
				fileIndex = fIndex
			}
		}

		resumeIndex := -1
		if resume != "" {
			rIndex, err := strconv.Atoi(resume)
			if err == nil && rIndex >= 0 {
				resumeIndex = rIndex
			}
		}

		tmdbId := -1
		if tmdb != "" {
			id, err := strconv.Atoi(tmdb)
			if err == nil && id >= 0 {
				tmdbId = id
			}
		}

		runTime := -1
		if tmdb != "" {
			runtimeInt, err := strconv.Atoi(runtime)
			if err == nil && runtimeInt >= 0 {
				runTime = runtimeInt
			}
		}

		params := bittorrent.BTPlayerParams{
			URI: uri,
			FileIndex: fileIndex,
			ResumeIndex: resumeIndex,
			ContentType: contentType,
			TMDBId: tmdbId,
			Runtime: runTime,
		}

		player := bittorrent.NewBTPlayer(btService, params)
		if player.Buffer() != nil {
			return
		}

		rUrl, _ := url.Parse(fmt.Sprintf("%s/files/%s", util.GetHTTPHost(), player.PlayURL()))
		ctx.Redirect(302, rUrl.String())
	}
}

func AddTorrent(ctx *gin.Context) {
	retval := xbmc.DialogInsert()
	if retval["path"] == "" {
		return
	}
	xbmc.PlayURL(UrlQuery(UrlForXBMC("/play"), "uri", retval["path"]))
}
