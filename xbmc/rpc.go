package xbmc

import "github.com/steeve/pulsar/jsonrpc"

type Args []interface{}
type Object map[string]interface{}

var Results map[string]chan interface{}

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
