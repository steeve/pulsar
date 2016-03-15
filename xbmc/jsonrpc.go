package xbmc

import (
	"net"

	"github.com/scakemyer/quasar/jsonrpc"
)

type Args []interface{}
type Object map[string]interface{}

var Results map[string]chan interface{}

var (
	XBMCJSONRPCHosts = []string{
		net.JoinHostPort("::1", "9090"),
		net.JoinHostPort("127.0.0.1", "9090"),
	}
	XBMCExJSONRPCHosts = []string{
		net.JoinHostPort("::1", "65252"),
		net.JoinHostPort("127.0.0.1", "65252"),
	}
)

func getConnection(hosts ...string) (net.Conn, error) {
	var err error

	for _, host := range hosts {
		c, err := net.Dial("tcp", host)
		if err == nil {
			return c, nil
		}
	}

	return nil, err
}

func executeJSONRPC(method string, retVal interface{}, args []interface{}) error {
	if args == nil {
		args = Args{}
	}
	conn, err := getConnection(XBMCJSONRPCHosts...)
	if err != nil {
		log.Error(err.Error())
		Notify("Quasar", "executeJSONRPC failed, check your logs.", "")
	}
	defer conn.Close()

	client := jsonrpc.NewClient(conn)
	return client.Call(method, args, retVal)
}

func executeJSONRPCEx(method string, retVal interface{}, args []interface{}) error {
	if args == nil {
		args = Args{}
	}
	conn, err := getConnection(XBMCExJSONRPCHosts...)
	if err != nil {
		log.Error(err.Error())
		Notify("Quasar", "executeJSONRPCEx failed, check your logs.", "")
	}
	defer conn.Close()

	client := jsonrpc.NewClient(conn)
	return client.Call(method, args, retVal)
}
