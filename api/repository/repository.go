package repository

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/google/go-github/github"
	"github.com/op/go-logging"
	"github.com/steeve/pulsar/xbmc"
)

const (
	githubUserContentURL = "https://raw.githubusercontent.com/%s/%s/%s"
	zipballURL           = "https://github.com/%s/%s/archive/%s.zip"
)

var (
	addonZipRE       = regexp.MustCompile(`[\w]+\.[\w]+(\.[\w]+)?-\d+\.\d+\.\d+\.zip`)
	addonChangelogRE = regexp.MustCompile(`changelog-\d+.\d+.\d+.txt`)
	log              = logging.MustGetLogger("repository")
)

func getLastTag(user string, repository string) (string, string) {
	client := github.NewClient(nil)
	tags, _, _ := client.Repositories.ListTags(user, repository, nil)
	if len(tags) > 0 {
		lastTag := tags[0]
		log.Info("Last tag: %s - %s", *lastTag.Name, *lastTag.Commit.SHA)
		return *lastTag.Name, *lastTag.Commit.SHA
	}
	log.Info("Unable to find a last tag, using master.")
	return "", "master"
}

func getReleaseByTag(user string, repository string, tagName string) *github.RepositoryRelease {
	client := github.NewClient(nil)
	releases, _, _ := client.Repositories.ListReleases(user, repository, nil)
	for _, release := range releases {
		if *release.TagName == tagName {
			return &release
		}
	}
	return nil
}

func getAddons(user string, repository string) (*xbmc.AddonList, error) {
	_, lastTagCommit := getLastTag(user, repository)
	resp, err := http.Get(fmt.Sprintf(githubUserContentURL, user, repository, lastTagCommit) + "/addon.xml")
	if err != nil {
		return nil, err
	}
	addon := xbmc.Addon{}
	xml.NewDecoder(resp.Body).Decode(&addon)
	return &xbmc.AddonList{
		Addons: []xbmc.Addon{addon},
	}, nil
}

func GetAddonsXML(ctx *gin.Context) {
	user := ctx.Params.ByName("user")
	repository := ctx.Params.ByName("repository")
	addons, err := getAddons(user, repository)
	if err != nil {
		ctx.Fail(404, errors.New("Unable to retrieve the remote's addon.xml file."))
	}
	ctx.XML(200, addons)
}

func GetAddonsXMLChecksum(ctx *gin.Context) {
	user := ctx.Params.ByName("user")
	repository := ctx.Params.ByName("repository")
	addons, err := getAddons(user, repository)
	if err != nil {
		ctx.Error(errors.New("Unable to retrieve the remote's addon.xml file."), nil)
	}
	hasher := md5.New()
	xml.NewEncoder(hasher).Encode(addons)
	ctx.String(200, hex.EncodeToString(hasher.Sum(nil)))
}

func GetAddonFiles(ctx *gin.Context) {
	user := ctx.Params.ByName("user")
	repository := ctx.Params.ByName("repository")
	filepath := ctx.Params.ByName("filepath")[1:] // strip the leading "/"

	lastTagName, lastTagCommit := getLastTag(user, repository)

	switch filepath {
	case "fanart.jpg":
		fallthrough
	case "icon.png":
		ctx.Redirect(302, fmt.Sprintf(githubUserContentURL+"/"+filepath, user, repository, lastTagCommit))
		return
	}

	switch {
	case addonZipRE.MatchString(filepath):
		addonZipURL(ctx, user, repository, lastTagName, lastTagCommit)
	case addonChangelogRE.MatchString(filepath):
		addonChangelog(ctx, user, repository, lastTagName, lastTagCommit)
	}
}

func addonZipURL(ctx *gin.Context, user string, repository string, lastTagName string, lastTagCommit string) {
	release := getReleaseByTag(user, repository, lastTagName)
	if release == nil {
		ctx.Redirect(302, fmt.Sprintf(zipballURL, user, repository, lastTagCommit))
		return
	}

	client := github.NewClient(nil)
	assets, _, _ := client.Repositories.ListReleaseAssets(user, repository, *release.ID, nil)
	for _, asset := range assets {
		if addonZipRE.MatchString(*asset.Name) {
			ctx.Redirect(302, *asset.BrowserDownloadUrl)
			return
		}
	}

	ctx.Fail(404, errors.New("Unable to find the specified file."))
}

func addonChangelog(ctx *gin.Context, user string, repository string, lastTagName string, lastTagCommit string) {
	ctx.String(200, "Pulsar Repository Changelog will go here.")
}
