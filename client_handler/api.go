package client_handler

import (
	"github.com/gonet2/agent/misc/packet"
	. "github.com/gonet2/agent/types"
)

// 协议号与对应的处理函数的映射
var Handlers = map[int16]func(*Session, *packet.Packet) []byte{
	1:   P_heart_beat_req,    // 心跳包
	10:  P_user_login_req,    // 登陆
	30:  P_get_seed_req,      // 请求通信加密种子
}

// 对外暴露的数据结构
var Code = map[string]int16{
	"heart_beat_req":         1,   // 心跳包
	"heart_beat_ack":         2,   // 心跳包回复
	"user_login_req":         10,  // 登陆
	"user_login_succeed_ack": 11,  // 登陆成功
	"user_login_faild_ack":   12,  // 登陆失败
	"get_seed_req":           30,  // 请求通信加密种子
	"get_seed_ack":           31,  // 回应通信加密种子
}
