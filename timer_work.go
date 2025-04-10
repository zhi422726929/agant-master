package main

import (
	log "github.com/sirupsen/logrus"
)

import (
	. "github.com/gonet2/agent/types"
)

// Player 1-minute timer
func timer_work(sess *Session, out *Buffer) {
	defer func() {
		sess.PacketCount1Min = 0
	}()

	// Packet frequency control, kick out if RPS is too high
	if sess.PacketCount1Min > rpmLimit {
		sess.Flag |= SESS_KICKED_OUT
		log.WithFields(log.Fields{
			"userid":  sess.UserId,
			"count1m": sess.PacketCount1Min,
			"total":   sess.PacketCount,
		}).Error("RPM")
		return
	}
}
