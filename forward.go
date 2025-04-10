package main

import (
	"errors"

	log "github.com/sirupsen/logrus"
)

import (
	pb "github.com/gonet2/agent/pb"
	. "github.com/gonet2/agent/types"
)

var (
	ERROR_STREAM_NOT_OPEN = errors.New("stream not opened yet")
)

// forward messages to game server
func forward(sess *Session, p []byte) error {
	frame := &pb.Game_Frame{
		Type:    pb.Game_Message,
		Message: p,
	}

	// check stream
	if sess.Stream == nil {
		return ERROR_STREAM_NOT_OPEN
	}

	// forward the frame to game
	if err := sess.Stream.Send(frame); err != nil {
		log.Error(err)
		return err
	}
	return nil
}









