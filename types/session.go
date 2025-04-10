package types

import (
	"crypto/rc4"
	"net"
	"time"
)

import (
	pb "github.com/gonet2/agent/pb"
)

const (
	SESS_KEYEXCG    = 0x1 // Key exchange completed
	SESS_ENCRYPT    = 0x2 // Encryption enabled
	SESS_KICKED_OUT = 0x4 // Kicked out
	SESS_AUTHORIZED = 0x8 // Authorized access
)

type Session struct {
	IP      net.IP
	MQ      chan pb.Game_Frame          // Async messages back to client
	Encoder *rc4.Cipher                 // Encryptor
	Decoder *rc4.Cipher                 // Decryptor
	UserId  int32                       // Player ID
	GSID    string                      // Game server ID; e.g.: game1,game2
	Stream  pb.GameService_StreamClient // Game server data stream
	Die     chan struct{}               // Session close signal

	// Session flags
	Flag int32

	// Time related
	ConnectTime    time.Time // TCP connection established time
	PacketTime     time.Time // Current packet arrival time
	LastPacketTime time.Time // Previous packet arrival time

	PacketCount     uint32 // Count received packets to avoid malicious packets
	PacketCount1Min int    // Packets per minute, used for RPM checks
}
