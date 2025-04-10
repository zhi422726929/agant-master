package client_handler

import (
	"crypto/rc4"
	"fmt"
	"io"
	"math/big"

	"golang.org/x/net/context"
	"google.golang.org/grpc/metadata"

	"github.com/gonet2/agent/misc/crypto/dh"
	"github.com/gonet2/agent/misc/packet"
	"github.com/gonet2/agent/services"

	log "github.com/sirupsen/logrus"

	pb "github.com/gonet2/agent/pb"

	. "github.com/gonet2/agent/types"
)

// Heartbeat package
func P_heart_beat_req(sess *Session, reader *packet.Packet) []byte {
	tbl, _ := PKT_auto_id(reader)
	return packet.Pack(Code["heart_beat_ack"], tbl, nil)
}

// Key exchange
// Encryption method: DH+RC4
// Note: The complete encryption process includes RSA+DH+RC4
// 1. RSA is used to verify the server's authenticity (omitted here)
// 2. DH is used to negotiate a secure KEY over an insecure channel
// 3. RC4 is used for stream encryption
func P_get_seed_req(sess *Session, reader *packet.Packet) []byte {
	tbl, _ := PKT_seed_info(reader)
	// KEY1
	X1, E1 := dh.DHExchange()
	KEY1 := dh.DHKey(X1, big.NewInt(int64(tbl.F_client_send_seed)))

	// KEY2
	X2, E2 := dh.DHExchange()
	KEY2 := dh.DHKey(X2, big.NewInt(int64(tbl.F_client_receive_seed)))

	ret := S_seed_info{int32(E1.Int64()), int32(E2.Int64())}
	// Server encryption seed is client decryption seed
	encoder, err := rc4.NewCipher([]byte(fmt.Sprintf("%v%v", SALT, KEY2)))
	if err != nil {
		log.Error(err)
		return nil
	}
	decoder, err := rc4.NewCipher([]byte(fmt.Sprintf("%v%v", SALT, KEY1)))
	if err != nil {
		log.Error(err)
		return nil
	}
	sess.Encoder = encoder
	sess.Decoder = decoder
	sess.Flag |= SESS_KEYEXCG
	return packet.Pack(Code["get_seed_ack"], ret, nil)
}

// Player login process
func P_user_login_req(sess *Session, reader *packet.Packet) []byte {
	// TODO: Login authentication
	// Simple authentication can be done directly in the agent, usually companies have a user center server for authentication
	sess.UserId = 1

	// TODO: Select GAME server
	// Service selection strategy depends on business requirements, for example, small servers can be fixed to a certain server, large servers can use HASH or consistent HASH
	sess.GSID = DEFAULT_GSID

	// Connect to the selected GAME server
	conn := services.GetServiceWithId("game-10000", sess.GSID)
	if conn == nil {
		log.Error("cannot get game service:", sess.GSID)
		return nil
	}
	cli := pb.NewGameServiceClient(conn)

	// Open stream to the game server
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(map[string]string{"userid": fmt.Sprint(sess.UserId)}))
	stream, err := cli.Stream(ctx)
	if err != nil {
		log.Error(err)
		return nil
	}
	sess.Stream = stream

	// Goroutine for reading messages returned by GAME
	fetcher_task := func(sess *Session) {
		for {
			in, err := sess.Stream.Recv()
			if err == io.EOF { // Stream closed
				log.Debug(err)
				return
			}
			if err != nil {
				log.Error(err)
				return
			}
			select {
			case sess.MQ <- *in:
			case <-sess.Die:
			}
		}
	}
	go fetcher_task(sess)
	return packet.Pack(Code["user_login_succeed_ack"], S_user_snapshot{F_uid: sess.UserId}, nil)
}
