package xbmc

import "github.com/steeve/pulsar/jsonrpc"

type Args []interface{}
type Object map[string]interface{}

const (
	XBMCDefaultJSONRPCHost = "[::1]:9090"
	XBMCExJSONRPCHost      = "localhost:9091"
)

func executeJSONRPC(method string, retVal interface{}, args []interface{}) error {
	if args == nil {
		args = Args{}
	}
	c, err := jsonrpc.Dial("tcp", XBMCDefaultJSONRPCHost)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	return c.Call(method, args, retVal)
}

func executeJSONRPCEx(method string, retVal interface{}, args []interface{}) error {
	if args == nil {
		args = Args{}
	}
	c, err := jsonrpc.Dial("tcp", XBMCExJSONRPCHost)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	return c.Call(method, args, retVal)
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
	executeJSONRPCEx("titan_keyboard", &retVal, args)
	return retVal
}

func ListDialog(title string, items ...string) int {
	retVal := -1
	executeJSONRPCEx("titan_dialog", &retVal, Args{title, items})
	return retVal
}
