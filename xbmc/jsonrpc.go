package xbmc

import (
	"net"

	"github.com/steeve/pulsar/jsonrpc"
)

type Args []interface{}
type Object map[string]interface{}

var Results map[string]chan interface{}

const (
	XBMCDefaultJSONRPCHost = "localhost:9090"
	XBMCExJSONRPCHost      = "localhost:65252"
)

func executeJSONRPC(method string, retVal interface{}, args []interface{}) error {
	if args == nil {
		args = Args{}
	}
	d := net.Dialer{DualStack: true}
	c, err := d.Dial("tcp", XBMCDefaultJSONRPCHost)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	client := jsonrpc.NewClient(c)
	return client.Call(method, args, retVal)
}

func executeJSONRPCEx(method string, retVal interface{}, args []interface{}) error {
	if args == nil {
		args = Args{}
	}
	d := net.Dialer{DualStack: true}
	c, err := d.Dial("tcp", XBMCExJSONRPCHost)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	client := jsonrpc.NewClient(c)
	return client.Call(method, args, retVal)
}
