package xbmc

type DialogProgress struct {
	hWnd int64
}

func NewDialogProgress(title, line1, line2, line3 string) *DialogProgress {
	retVal := int64(-1)
	executeJSONRPCEx("DialogProgress_Create", &retVal, Args{title, line1, line2, line3})
	if retVal < 0 {
		return nil
	}
	return &DialogProgress{
		hWnd: retVal,
	}
}

func (dp *DialogProgress) Update(percent int, line1, line2, line3 string) {
	retVal := -1
	executeJSONRPCEx("DialogProgress_Update", &retVal, Args{dp.hWnd, percent, line1, line2, line3})
}

func (dp *DialogProgress) IsCanceled() bool {
	retVal := 0
	executeJSONRPCEx("DialogProgress_IsCanceled", &retVal, Args{dp.hWnd})
	return retVal != 0
}

func (dp *DialogProgress) Close() {
	retVal := -1
	executeJSONRPCEx("DialogProgress_Close", &retVal, Args{dp.hWnd})
}

func Notify(args ...interface{}) {
	var retVal string
	executeJSONRPC("GUI.ShowNotification", &retVal, args)
}

func InfoLabels(labels ...string) map[string]string {
	var retVal map[string]string
	executeJSONRPC("XBMC.GetInfoLabels", &retVal, Args{labels})
	return retVal
}

func InfoLabel(label string) string {
	labels := InfoLabels(label)
	return labels[label]
}

func Keyboard(args ...interface{}) string {
	var retVal string
	executeJSONRPCEx("Keyboard", &retVal, args)
	return retVal
}

func ListDialog(title string, items ...string) int {
	retVal := -1
	executeJSONRPCEx("Dialog_Select", &retVal, Args{title, items})
	return retVal
}

func PlayerGetPlayingFile() string {
	retVal := ""
	executeJSONRPCEx("Player_GetPlayingFile", &retVal, nil)
	return retVal
}

func PlayerIsPlaying() bool {
	retVal := 0
	executeJSONRPCEx("Player_IsPlaying", &retVal, nil)
	return retVal != 0
}
