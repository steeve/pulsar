package repository

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/go-github/github"
	"github.com/op/go-logging"
	"github.com/scakemyer/quasar/xbmc"
	"github.com/scakemyer/quasar/config"
)

const (
	githubUserContentURL = "https://raw.githubusercontent.com/%s/%s/%s"
	tarballURL           = "https://github.com/%s/%s/archive/%s.tar.gz"
)

var (
	mainReleaseRE    = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	addonZipRE       = regexp.MustCompile(`[\w]+\.[\w]+(\.[\w]+)?-\d+\.\d+\.\d+(-[\w]+\.\d+)?\.zip`)
	addonChangelogRE = regexp.MustCompile(`changelog-\d+.\d+.\d+(-[\w]+\.\d+)?.txt`)
	log              = logging.MustGetLogger("repository")
)

func getLastTag(user string, repository string) (string, string) {
	client := github.NewClient(nil)
	tags, _, _ := client.Repositories.ListTags(user, repository, nil)
	if len(tags) > 0 {
		lastTag := tags[0]
		if config.Get().PreReleaseUpdates == false {
			log.Info("Looking for last main release...")
			for _, tag := range tags {
				if mainReleaseRE.MatchString(*tag.Name) {
					log.Info("%s matches", *tag.Name)
					lastTag = tag
					break
				}
			}
		}
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

	lastTagName, lastTagCommit := getLastTag(user, repository)

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
		ctx.Redirect(302, fmt.Sprintf(githubUserContentURL+"/"+filepath, user, repository, lastTagCommit))
		return
	}

	switch {
	case addonZipRE.MatchString(filepath):
		addonZip(ctx, user, repository, lastTagName, lastTagCommit)
	case addonChangelogRE.MatchString(filepath):
		addonChangelog(ctx, user, repository, lastTagName, lastTagCommit)
	}
}

func addonZip(ctx *gin.Context, user string, repository string, lastTagName string, lastTagCommit string) {
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
	}

	resp, err := http.Get(fmt.Sprintf(tarballURL, user, repository, lastTagCommit))
	if err != nil {
		ctx.AbortWithError(500, err)
	}
	gzReader, _ := gzip.NewReader(resp.Body)
	tarReader := tar.NewReader(gzReader)
	zipWriter := zip.NewWriter(ctx.Writer)
	defer zipWriter.Close()
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			ctx.AbortWithError(500, err)
		}
		// somehow Github packs this file too
		if hdr.Name == "pax_global_header" {
			continue
		}
		// make sure the top level directory doesn't end with the tag/commit
		newFileName := strings.Replace(hdr.Name, fmt.Sprintf("%s-%s/", repository, lastTagCommit), fmt.Sprintf("%s/", repository), 1)
		newFile, err := zipWriter.Create(newFileName)
		if err != nil {
			ctx.AbortWithError(500, err)
		}
		if _, err := io.Copy(newFile, tarReader); err != nil {
			ctx.AbortWithError(500, err)
		}
	}
}

func addonChangelog(ctx *gin.Context, user string, repository string, lastTagName string, lastTagCommit string) {
	ctx.String(200, "Quasar Repository Changelog will go here.")
}
