package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/gonet2/agent/client_handler"
	"github.com/gonet2/agent/misc/packet"
	. "github.com/gonet2/agent/types"
)

// 根据消息类型处理消息
func route(sess *Session, p []byte) []byte {
	// 解包
	reader := packet.Reader(p)
	
	// 读取协议号
	b, err := reader.ReadS16()
	if err != nil {
		log.Error("read protocol error:", err)
		return nil
	}

	// 协议号处理
	handle := client_handler.Handlers[b]
	if handle != nil {
		ret := handle(sess, reader)
		return ret
	}

	log.Warningf("no handler for protocol: %v", b)
	return nil
}
