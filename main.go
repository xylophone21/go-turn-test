// Package main go 实现的压测工具
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/pion/logging"
	"github.com/xylophone21/go-turn-test/dispose"
)

var (
	connections  uint64        = 5
	duration     time.Duration = time.Second * 30
	packageSize  int           = 1024
	packageWait  time.Duration = time.Second
	statLogLvl   int           = int(logging.LogLevelInfo)
	reqLogLvl    int           = int(logging.LogLevelError)
	is2CloudMode bool          = false
	isAwsMode    bool          = false
	stunServer   string        = ""
	turnServer   string        = "hellohui.space:3478"
	username     string        = ""
	password     string        = ""
)

func init() {
	flag.Uint64Var(&connections, "c", connections, "Number of TURN connections")
	flag.DurationVar(&duration, "d", duration, "Duration of test")
	flag.IntVar(&packageSize, "s", packageSize, "Package size to send")
	flag.DurationVar(&packageWait, "w", packageWait, "Duration per each send")
	flag.IntVar(&statLogLvl, "statlog", statLogLvl, "Log level of statistics")
	flag.IntVar(&reqLogLvl, "reqlog", reqLogLvl, "Log level of request")
	flag.BoolVar(&is2CloudMode, "2cloud", is2CloudMode, "Using cloud2cloud turn mode")
	flag.BoolVar(&isAwsMode, "aws", isAwsMode, "Using AWS turn server")
	flag.StringVar(&stunServer, "stun", stunServer, "Stun server url")
	flag.StringVar(&turnServer, "turn", stunServer, "Turn server url")
	flag.StringVar(&username, "u", username, "Username of turn server")
	flag.StringVar(&password, "p", password, "Password of turn server")

	// 解析参数
	flag.Parse()
}

func main() {
	req := &dispose.DisposeRequestST{
		ChanCount:      connections,
		Duration:       duration,
		PackageSize:    int32(packageSize),
		PackageWait:    packageWait,
		StatLogLvl:     statLogLvl,
		ReqLogLvl:      reqLogLvl,
		StunServerAddr: stunServer,
		TurnServerAddr: turnServer,
		Username:       username,
		Password:       password,
	}

	var mode string
	if is2CloudMode {
		req.Mode = dispose.MODE_2CLOUD
		mode = "2 cloud mode"
	} else {
		req.Mode = dispose.MODE_1CLOUD
		mode = "1 cloud mode"
	}

	var server string
	if isAwsMode {
		req.Source = dispose.SOURCE_AWS
		server = "{get from aws}"
	} else {
		req.Source = dispose.SOURCE_BASE
		server = turnServer
	}

	//todo
	//1. added deviceid and token
	//2. added paramters check for each mode
	//3. put debug user & psw to config

	fmt.Printf("Start request %v connections to %v by %v\n", connections, server, mode)

	err := dispose.Dispose(req)
	if err != nil {
		fmt.Printf("Run error:%v\n", err)
	}
}
