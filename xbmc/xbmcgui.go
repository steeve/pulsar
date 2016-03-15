package xbmc

type DialogProgress struct {
	hWnd int64
}

type DialogProgressBG struct {
	hWnd int64
}

type OverlayStatus struct {
	hWnd int64
}

func DialogInsert() map[string]string {
	var retVal map[string]string
	executeJSONRPCEx("DialogInsert", &retVal, nil)
	return retVal
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


func NewDialogProgressBG(title, message string) *DialogProgressBG {
	retVal := int64(-1)
	executeJSONRPCEx("DialogProgressBG_Create", &retVal, Args{title, message})
	if retVal < 0 {
		return nil
	}
	return &DialogProgressBG{
		hWnd: retVal,
	}
}

func (dp *DialogProgressBG) Update(percent int, message string) {
	retVal := -1
	executeJSONRPCEx("DialogProgress_Update", &retVal, Args{dp.hWnd, percent, message})
}

func (dp *DialogProgressBG) IsFinished() bool {
	retVal := 0
	executeJSONRPCEx("DialogProgressBG_IsFinished", &retVal, Args{dp.hWnd})
	return retVal != 0
}

func (dp *DialogProgressBG) Close() {
	retVal := -1
	executeJSONRPCEx("DialogProgressBG_Close", &retVal, Args{dp.hWnd})
}


func NewOverlayStatus() *OverlayStatus {
	retVal := int64(-1)
	executeJSONRPCEx("OverlayStatus_Create", &retVal, Args{})
	if retVal < 0 {
		return nil
	}
	return &OverlayStatus{
		hWnd: retVal,
	}
}

func (ov *OverlayStatus) Update(percent int, line1, line2, line3 string) {
	retVal := -1
	executeJSONRPCEx("OverlayStatus_Update", &retVal, Args{ov.hWnd, percent, line1, line2, line3})
}

func (ov *OverlayStatus) Show() {
	retVal := -1
	executeJSONRPCEx("OverlayStatus_Show", &retVal, Args{ov.hWnd})
}

func (ov *OverlayStatus) Hide() {
	retVal := -1
	executeJSONRPCEx("OverlayStatus_Hide", &retVal, Args{ov.hWnd})
}

func (ov *OverlayStatus) Close() {
	retVal := -1
	executeJSONRPCEx("OverlayStatus_Close", &retVal, Args{ov.hWnd})
}


func Notify(header string, message string, image string) {
	var retVal string
	//executeJSONRPC("GUI.ShowNotification", &retVal, args)
	executeJSONRPCEx("Notify", &retVal, Args{header, message, image})
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

func Dialog(title string, message string) bool {
	retVal := 0
	executeJSONRPCEx("Dialog", &retVal, Args{title, message})
	return retVal != 0
}

func DialogConfirm(title string, message string) bool {
	retVal := 0
	executeJSONRPCEx("Dialog_Confirm", &retVal, Args{title, message})
	return retVal != 0
}

func ListDialog(title string, items ...string) int {
	retVal := -1
	executeJSONRPCEx("Dialog_Select", &retVal, Args{title, items})
	return retVal
}

func ListDialogLarge(title string, subject string, items ...string) int {
	retVal := -1
	executeJSONRPCEx("Dialog_Select_Large", &retVal, Args{title, subject, items})
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

func PlayerIsPaused() bool {
	retVal := 0
	executeJSONRPCEx("Player_IsPaused", &retVal, nil)
	return retVal != 0
}

func GetWatchTimes() map[string]string {
	var retVal map[string]string
	executeJSONRPCEx("Player_WatchTimes", &retVal, nil)
	return retVal
}

func CloseAllDialogs() bool {
	retVal := 0
	executeJSONRPCEx("Dialog_CloseAll", &retVal, nil)
	return retVal != 0
}
