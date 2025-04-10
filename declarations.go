package main

import (
	"sync"
	
	cli "github.com/urfave/cli/v2"
	"github.com/gonet2/agent/services"
)

// Global variables
var (
	wg  sync.WaitGroup
	die chan struct{}
	rpmLimit int // 用于initTimer函数
)

// Function declarations
func startup(c *cli.Context) {
	go sig_handler()
	services.Init(c)
}

func initTimer(rpm_limit int) {
	rpmLimit = rpm_limit
	die = make(chan struct{})
}

// PACKET_LIMIT constant if needed
const PACKET_LIMIT = 65535
