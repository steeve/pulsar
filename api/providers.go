package api

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/scakemyer/quasar/config"
	"github.com/scakemyer/quasar/xbmc"
)

type Addon struct {
	ID      string
	Name    string
	Version string
	Enabled bool
	Status  int
}

type ByEnabled []Addon
func (a ByEnabled) Len() int           { return len(a) }
func (a ByEnabled) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByEnabled) Less(i, j int) bool { return a[i].Enabled }

type ByStatus []Addon
func (a ByStatus) Len() int           { return len(a) }
func (a ByStatus) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByStatus) Less(i, j int) bool { return a[i].Status < a[j].Status }

func getProviders() []Addon {
	list := make([]Addon, 0)
	for _, addon := range xbmc.GetAddons("xbmc.python.script", "executable", "all", []string{"name", "version", "enabled"}).Addons {
		if strings.HasPrefix(addon.ID, "script.quasar.") {
			list = append(list, Addon{
				ID: addon.ID,
				Name: addon.Name,
				Version: addon.Version,
				Enabled: addon.Enabled,
				Status: xbmc.AddonCheck(addon.ID),
			})
		}
	}
	sort.Sort(ByStatus(list))
	sort.Sort(ByEnabled(list))
	return list
}

func ProviderList(ctx *gin.Context) {
	providers := getProviders()

	items := make(xbmc.ListItems, 0, len(providers))
	for _, provider := range providers {
		status := "[COLOR FF009900]Ok[/COLOR]"
		if provider.Status > 0 {
			status = "[COLOR FF999900]Fail[/COLOR]"
		}

		enabled := "[COLOR FF009900]Enabled[/COLOR]"
		defaultAction := UrlForXBMC("/provider/%s/settings", provider.ID)
		if provider.Enabled == false {
			enabled = "[COLOR FF990000]Disabled[/COLOR]"
			UrlForXBMC("/provider/%s/enable", provider.ID)
		}

		item := &xbmc.ListItem{
			Label:      fmt.Sprintf("%s - %s - %s %s", status, enabled, provider.Name, provider.Version),
			Path:       defaultAction,
			IsPlayable: false,
		}
		item.ContextMenu = [][]string{
			[]string{"LOCALIZE[30242]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/provider/%s/check", provider.ID))},
		}
		if provider.Enabled {
			item.ContextMenu = append(item.ContextMenu,
				[]string{"LOCALIZE[30241]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/provider/%s/disable", provider.ID))},
				[]string{"LOCALIZE[30244]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/provider/%s/settings", provider.ID))},
			)
		} else {
			item.ContextMenu = append(item.ContextMenu,
				[]string{"LOCALIZE[30240]", fmt.Sprintf("XBMC.RunPlugin(%s)", UrlForXBMC("/provider/%s/enable", provider.ID))},
			)
		}
		items = append(items, item)
	}

	ctx.JSON(200, xbmc.NewView("", items))
}

func ProviderSettings(ctx *gin.Context) {
	addonId := ctx.Params.ByName("provider")
	xbmc.AddonSettings(addonId)
	ctx.String(200, "")
}

func ProviderCheck(ctx *gin.Context) {
	addonId := ctx.Params.ByName("provider")
	failures := xbmc.AddonCheck(addonId)
	translated := xbmc.GetLocalizedString(30243)
	xbmc.Notify("Quasar", fmt.Sprintf("%s: %d", translated, failures), config.AddonIcon())
	ctx.String(200, "")
}

func ProviderFailure(ctx *gin.Context) {
	addonId := ctx.Params.ByName("provider")
	xbmc.AddonFailure(addonId)
	ctx.String(200, "")
}

func ProviderEnable(ctx *gin.Context) {
	addonId := ctx.Params.ByName("provider")
	xbmc.SetAddonEnabled(addonId, true)
	path := xbmc.InfoLabel("Container.FolderPath")
	if path == "plugin://plugin.video.quasar/provider/" {
		xbmc.Refresh()
	}
	ctx.String(200, "")
}

func ProviderDisable(ctx *gin.Context) {
	addonId := ctx.Params.ByName("provider")
	xbmc.SetAddonEnabled(addonId, false)
	path := xbmc.InfoLabel("Container.FolderPath")
	if path == "plugin://plugin.video.quasar/provider/" {
		xbmc.Refresh()
	}
	ctx.String(200, "")
}
