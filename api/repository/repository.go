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
	"github.com/scakemyer/quasar/xbmc"
	"github.com/scakemyer/quasar/config"
)

const (
	githubUserContentURL = "https://raw.githubusercontent.com/%s/%s/%s"
	tarballURL           = "https://github.com/%s/%s/archive/%s.tar.gz"
	releaseChangelog     = "[B]%s[/B] - %s\n%s\n\n"
)

var (
	addonZipRE       = regexp.MustCompile(`[\w]+\.[\w]+(\.[\w]+)?-\d+\.\d+\.\d+(-[\w]+\.\d+)?\.zip`)
	addonChangelogRE = regexp.MustCompile(`changelog.*\.txt`)
	log              = logging.MustGetLogger("repository")
)

func getLastRelease(user string, repository string) (string, string) {
	client := github.NewClient(nil)
	releases, _, _ := client.Repositories.ListReleases(user, repository, nil)
	if len(releases) > 0 {
		lastRelease := releases[0]
		if config.Get().PreReleaseUpdates == false {
			log.Info("Getting latest main release")
			latestRelease, _, _ := client.Repositories.GetLatestRelease(user, repository)
			lastRelease = *latestRelease
		}
		log.Info("Last release: %s on %s", *lastRelease.TagName, *lastRelease.TargetCommitish)
		return *lastRelease.TagName, *lastRelease.TargetCommitish
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
	_, lastReleaseBranch := getLastRelease(user, repository)
	resp, err := http.Get(fmt.Sprintf(githubUserContentURL, user, repository, lastReleaseBranch) + "/addon.xml")
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
		ctx.AbortWithError(404, errors.New("Unable to retrieve the remote's addon.xml file."))
	}
	ctx.XML(200, addons)
}

func GetAddonsXMLChecksum(ctx *gin.Context) {
	user := ctx.Params.ByName("user")
	repository := ctx.Params.ByName("repository")
	addons, err := getAddons(user, repository)
	if err != nil {
		ctx.Error(errors.New("Unable to retrieve the remote's addon.xml file."))
	}
	hasher := md5.New()
	xml.NewEncoder(hasher).Encode(addons)
	ctx.String(200, hex.EncodeToString(hasher.Sum(nil)))
}

func GetAddonFiles(ctx *gin.Context) {
	user := ctx.Params.ByName("user")
	repository := ctx.Params.ByName("repository")
	filepath := ctx.Params.ByName("filepath")[1:] // strip the leading "/"

	lastReleaseName, lastReleaseBranch := getLastRelease(user, repository)

	switch filepath {
	case "addons.xml":
		GetAddonsXML(ctx)
		return
	case "addons.xml.md5":
		GetAddonsXMLChecksum(ctx)
		return
	case "fanart.jpg":
		fallthrough
	case "icon.png":
		ctx.Redirect(302, fmt.Sprintf(githubUserContentURL+"/"+filepath, user, repository, lastReleaseBranch))
		return
	}

	switch {
	case addonZipRE.MatchString(filepath):
		addonZip(ctx, user, repository, lastReleaseName)
	case addonChangelogRE.MatchString(filepath):
		addonChangelog(ctx, user, repository, lastReleaseName)
	default:
		ctx.AbortWithError(404, errors.New(filepath))
	}
}

func addonZip(ctx *gin.Context, user string, repository string, lastTagName string) {
	release := getReleaseByTag(user, repository, lastTagName)
	// if there a release with an asset that matches a addon zip, use it
	if release != nil {
		client := github.NewClient(nil)
		assets, _, _ := client.Repositories.ListReleaseAssets(user, repository, *release.ID, nil)
		for _, asset := range assets {
			if addonZipRE.MatchString(*asset.Name) {
				ctx.Redirect(302, *asset.BrowserDownloadURL)
				return
			}
		}
	} else {
		ctx.AbortWithError(404, errors.New("Release assets not yet available."))
	}
}

func addonChangelog(ctx *gin.Context, user string, repository string, lastTagName string) {
	log.Info("Fetching add-on changelog...")
	client := github.NewClient(nil)
	releases, _, _ := client.Repositories.ListReleases(user, repository, nil)
	changelog := "Quasar changelog\n======\n\n"
	for _, release := range releases {
		changelog += fmt.Sprintf(releaseChangelog, *release.TagName, release.PublishedAt.Format("Jan 2 2006"), *release.Body)
	}
	ctx.String(200, changelog)
}
