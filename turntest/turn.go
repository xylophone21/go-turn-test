package turntest

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"net"
	"strings"
	"time"

	"github.com/pion/logging"
	"github.com/pion/turn/v2"
)

const (
	minPackageSize = 128
	minPackageWait = time.Microsecond * 100
)

type TrunRequestST struct {
	Ctx            context.Context
	Log            logging.LeveledLogger
	ChanId         uint64
	PackageSize    int32
	PackageWait    time.Duration
	StunServerAddr string // STUN server address (e.g. "stun.abc.com:3478")
	TurnServerAddr string // TURN server addrees (e.g. "turn.abc.com:3478")
	Username       string
	Password       string
	PublicIPTst    bool // AWS TURN doest not ignored port in create permission and and will not response for BindingRequest, so we have to test it with public IP
}

type relayClient struct {
	Conn      net.PacketConn
	Client    *turn.Client
	RelayConn net.PacketConn
}

func TrunRequest(req *TrunRequestST) error {
	if req == nil {
		err := fmt.Errorf("[TrunRequest-unkonw]req nil")
		return err
	}

	if req.Ctx == nil || req.Log == nil || req.PackageSize < minPackageSize || req.PackageWait < minPackageWait || req.TurnServerAddr == "" {
		err := fmt.Errorf("[TrunRequest-%d]Paramters error", req.ChanId)
		return err
	}

	if req.StunServerAddr == "" {
		req.StunServerAddr = req.TurnServerAddr
	}

	for {
		doTrunRequest(req)

		// timeout or canceled, return
		select {
		case <-req.Ctx.Done():
			return nil

		// wait 100 Microsecond and retry
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}

func doTrunRequest(req *TrunRequestST) error {
	var lc net.ListenConfig
	conn, err := lc.ListenPacket(req.Ctx, "udp4", "0.0.0.0:0")
	if err != nil {
		req.Log.Warnf("[TrunRequest-%d]ListenPacket error:%s", req.ChanId, err)
		return err
	}
	defer conn.Close()

	cfg := &turn.ClientConfig{
		STUNServerAddr: req.StunServerAddr,
		TURNServerAddr: req.TurnServerAddr,
		Conn:           conn,
		Username:       req.Username,
		Password:       req.Password,
		Realm:          "go-turn-test",
		RTO:            time.Second,
	}
	client, err := turn.NewClient(cfg)
	if err != nil {
		req.Log.Warnf("[TrunRequest-%d]turn.NewClient error:%s", req.ChanId, err)
		return err
	}
	defer client.Close()

	// Start listening on the conn provided.
	err = client.Listen()
	if err != nil {
		req.Log.Warnf("[TrunRequest-%d]client.Listen() error:%s", req.ChanId, err)
		return err
	}

	relayConn, err := client.Allocate()
	if err != nil {
		req.Log.Warnf("[TrunRequest-%d]client.Allocate() error:%s", req.ChanId, err)
		return err
	}
	defer relayConn.Close()

	// Set up sender socket (pingerConn)
	var d net.Dialer
	senderConn, err := d.DialContext(req.Ctx, relayConn.LocalAddr().Network(), relayConn.LocalAddr().String())
	if err != nil {
		req.Log.Warnf("[TrunRequest-%d]d.DialContext error:%s", req.ChanId, err)
		return err
	}
	defer senderConn.Close()

	// Send BindingRequest to learn our external IP
	mappedAddr, err := client.SendBindingRequest()
	if err != nil {
		req.Log.Warnf("[TrunRequest-%d]client.SendBindingRequest() error:%s", req.ChanId, err)
		return err
	}

	// [workaround] server with pulibc ip will usually have a local ip but mapping all port to local ip
	// so use public ip and connection port
	if req.PublicIPTst {
		addrIp := strings.Split(mappedAddr.String(), ":")
		addrPort := strings.Split(senderConn.LocalAddr().String(), ":")
		mappedAddr, _ = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%s", addrIp[0], addrPort[1]))
	}

	// added mappedAddr (without port) to permission list in turn server
	_, err = relayConn.WriteTo([]byte(fmt.Sprintf("Hello-%d", req.ChanId)), mappedAddr)
	if err != nil {
		req.Log.Warnf("[TrunRequest-%d]relayConn.WriteTo error:%s", req.ChanId, err)
		return err
	}

	chanIdOffset := 0
	timeLenOffset := chanIdOffset + 8
	timeOffset := timeLenOffset + 4
	crcOffset := req.PackageSize - 8

	time.Sleep(500 * time.Millisecond)

	timeSend := time.Now()

	// Start read-loop on relayConn
	go func() {
		var byteRecv uint64 = 0
		recvBuf := make([]byte, req.PackageSize+32)
		for {
			n, _, err := relayConn.ReadFrom(recvBuf)
			if err != nil {
				req.Log.Warnf("[TrunRequest-%d]relayConn.ReadFrom error:%s", req.ChanId, err)
				return
			}

			if n != int(req.PackageSize) {
				req.Log.Warnf("[TrunRequest-%d]relayConn.ReadFrom len error,want %d got %d", req.ChanId, req.PackageSize, n)
			}

			chanId := binary.BigEndian.Uint64(recvBuf[chanIdOffset:])
			if chanId != req.ChanId {
				req.Log.Warnf("[TrunRequest-%d]chanId error:%d", req.ChanId, chanId)
			}

			timeLen := int(binary.BigEndian.Uint32(recvBuf[timeLenOffset:]))

			timeStr := string(recvBuf[timeOffset : timeOffset+timeLen])
			sentAt, err := time.Parse(time.RFC3339Nano, timeStr)
			if err != nil {
				req.Log.Warnf("[TrunRequest-%d]time.Parse error:%s", req.ChanId, err)
			}

			crc32Get := binary.BigEndian.Uint32(recvBuf[crcOffset:])
			crc32Sum := crc32.ChecksumIEEE(recvBuf[:crcOffset])
			if crc32Get != crc32Sum {
				req.Log.Warnf("[TrunRequest-%d]crc error, want %x got %x", req.ChanId, crc32Sum, crc32Get)
			}

			byteRecv += uint64(n)
			since := time.Since(timeSend).Seconds()

			delay := time.Since(sentAt).Milliseconds()

			if err == nil {
				if delay > 0 {
					req.Log.Tracef("[TrunRequest-%d] Recv %d kps delay=%d", req.ChanId, int(8*float64(byteRecv)/since/1024), delay)
				}
			}
		}
	}()

	sendBuf := make([]byte, req.PackageSize)
	rand.Read(sendBuf)

	var byteSend uint64 = 0
	for {
		binary.BigEndian.PutUint64(sendBuf[chanIdOffset:], req.ChanId)

		nowStr := time.Now().Format(time.RFC3339Nano)
		binary.BigEndian.PutUint32(sendBuf[timeLenOffset:], uint32(len(nowStr)))
		copy(sendBuf[timeOffset:], []byte(nowStr))

		crc32 := crc32.ChecksumIEEE(sendBuf[:crcOffset])
		binary.BigEndian.PutUint32(sendBuf[crcOffset:], crc32)

		_, err = senderConn.Write(sendBuf)
		if err != nil {
			req.Log.Warnf("[TrunRequest-%d]senderConn.WriteTo error:%s", req.ChanId, err)
			return err
		}
		byteSend += uint64(len(sendBuf))

		time.Sleep(req.PackageWait)

		since := time.Since(timeSend).Seconds()

		if since > 0 {
			req.Log.Tracef("[TrunRequest-%d] Send %d kps", req.ChanId, int(8*float64(byteSend)/since/1024))
		}
	}
}

// PeerA <--> RelayA  <---> RelayB  <---> PeerB
func TrunRequest2Cloud(req *TrunRequestST) error {
	if req == nil {
		err := fmt.Errorf("[TrunRequest2Cloud-unkonw]req nil")
		return err
	}

	if req.Ctx == nil || req.Log == nil || req.PackageSize < minPackageSize || req.PackageWait < minPackageWait || req.TurnServerAddr == "" {
		err := fmt.Errorf("[TrunRequest2Cloud-%d]Paramters error", req.ChanId)
		return err
	}

	if req.StunServerAddr == "" {
		req.StunServerAddr = req.TurnServerAddr
	}

	for {
		doTrunRequest2Cloud(req)

		// timeout or canceled, return
		select {
		case <-req.Ctx.Done():
			return nil

		// wait 100 Microsecond and retry
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}

func doTrunRequest2Cloud(req *TrunRequestST) error {
	relay1, err := allocRelayClient(req)
	if err != nil {
		return err
	}
	defer freeRelayClient(relay1)

	relay2, err := allocRelayClient(req)
	if err != nil {
		return err
	}
	defer freeRelayClient(relay2)

	// added mappedAddr (without port) to permission list in turn server
	_, err = relay1.RelayConn.WriteTo([]byte("Hello"), relay2.RelayConn.LocalAddr())
	if err != nil {
		req.Log.Warnf("[TrunRequest2Cloud-%d]relayConn.WriteTo error:%s", req.ChanId, err)
		return err
	}
	_, err = relay2.RelayConn.WriteTo([]byte("Hello"), relay1.RelayConn.LocalAddr())
	if err != nil {
		req.Log.Warnf("[TrunRequest2Cloud-%d]relayConn.WriteTo error:%s", req.ChanId, err)
		return err
	}

	chanIdOffset := 0
	timeLenOffset := chanIdOffset + 8
	timeOffset := timeLenOffset + 4
	crcOffset := req.PackageSize - 8

	// time.Sleep(500 * time.Millisecond)

	timeSend := time.Now()
	// Start read-loop on relayConn
	go func() {
		var byteRecv uint64 = 0
		recvBuf := make([]byte, req.PackageSize+32)
		for {
			n, _, err := relay2.RelayConn.ReadFrom(recvBuf)
			if err != nil {
				req.Log.Warnf("[TrunRequest2Cloud-%d]relayConn.ReadFrom error:%s", req.ChanId, err)
				return
			}

			if string(recvBuf[:n]) == "Hello" {
				continue
			}

			if n != int(req.PackageSize) {
				req.Log.Warnf("[TrunRequest2Cloud-%d]relayConn.ReadFrom len error,want %d got %d", req.ChanId, req.PackageSize, n)
			}

			chanId := binary.BigEndian.Uint64(recvBuf[chanIdOffset:])
			if chanId != req.ChanId {
				req.Log.Warnf("[TrunRequest2Cloud-%d]chanId error:%d", req.ChanId, chanId)
			}

			timeLen := int(binary.BigEndian.Uint32(recvBuf[timeLenOffset:]))

			timeStr := string(recvBuf[timeOffset : timeOffset+timeLen])
			sentAt, err := time.Parse(time.RFC3339Nano, timeStr)
			if err != nil {
				req.Log.Warnf("[TrunRequest2Cloud-%d]time.Parse error:%s", req.ChanId, err)
			}

			crc32Get := binary.BigEndian.Uint32(recvBuf[crcOffset:])
			crc32Sum := crc32.ChecksumIEEE(recvBuf[:crcOffset])
			if crc32Get != crc32Sum {
				req.Log.Warnf("[TrunRequest2Cloud-%d]crc error, want %x got %x", req.ChanId, crc32Sum, crc32Get)
			}

			byteRecv += uint64(n)
			since := time.Since(timeSend).Seconds()

			delay := time.Since(sentAt).Milliseconds()

			if err == nil {
				if delay > 0 {
					req.Log.Tracef("[TrunRequest2Cloud-%d] Recv %d kps delay=%d", req.ChanId, int(8*float64(byteRecv)/since/1024), delay)
				}
			}
		}
	}()

	sendBuf := make([]byte, req.PackageSize)
	rand.Read(sendBuf)

	var byteSend uint64 = 0
	for {
		binary.BigEndian.PutUint64(sendBuf[chanIdOffset:], req.ChanId)

		nowStr := time.Now().Format(time.RFC3339Nano)
		binary.BigEndian.PutUint32(sendBuf[timeLenOffset:], uint32(len(nowStr)))
		copy(sendBuf[timeOffset:], []byte(nowStr))

		crc32 := crc32.ChecksumIEEE(sendBuf[:crcOffset])
		binary.BigEndian.PutUint32(sendBuf[crcOffset:], crc32)

		_, err = relay1.RelayConn.WriteTo(sendBuf, relay2.RelayConn.LocalAddr())
		if err != nil {
			req.Log.Warnf("[TrunRequest2Cloud-%d]senderConn.WriteTo error:%s", req.ChanId, err)
			return err
		}
		byteSend += uint64(len(sendBuf))

		time.Sleep(req.PackageWait)

		since := time.Since(timeSend).Seconds()

		if since > 0 {
			req.Log.Tracef("[TrunRequest2Cloud-%d] Send %d kps", req.ChanId, int(8*float64(byteSend)/since/1024))
		}
	}
}

func allocRelayClient(req *TrunRequestST) (*relayClient, error) {
	var relay relayClient
	var err error
	defer func() {
		if err != nil {
			freeRelayClient(&relay)
		}
	}()

	var lc net.ListenConfig
	relay.Conn, err = lc.ListenPacket(req.Ctx, "udp4", "0.0.0.0:0")
	if err != nil {
		req.Log.Warnf("[TrunRequest2Cloud-%d]ListenPacket error:%s", req.ChanId, err)
		return nil, err
	}

	cfg := &turn.ClientConfig{
		STUNServerAddr: req.StunServerAddr,
		TURNServerAddr: req.TurnServerAddr,
		Conn:           relay.Conn,
		Username:       req.Username,
		Password:       req.Password,
		Realm:          "go-turn-test",
		RTO:            time.Second,
	}
	relay.Client, err = turn.NewClient(cfg)
	if err != nil {
		req.Log.Warnf("[TrunRequest2Cloud-%d]turn.NewClient error:%s", req.ChanId, err)
		return nil, err
	}

	// Start listening on the conn provided.
	err = relay.Client.Listen()
	if err != nil {
		req.Log.Warnf("[TrunRequest2Cloud-%d]client.Listen() error:%s", req.ChanId, err)
		return nil, err
	}

	relay.RelayConn, err = relay.Client.Allocate()
	if err != nil {
		req.Log.Warnf("[TrunRequest2Cloud-%d]client.Allocate() error:%s", req.ChanId, err)
		return nil, err
	}

	return &relay, nil
}

func freeRelayClient(relay *relayClient) {
	if relay == nil {
		return
	}

	if relay.Conn != nil {
		relay.Conn.Close()
		relay.Conn = nil
	}

	if relay.Client != nil {
		relay.Client.Close()
		relay.Client = nil
	}

	if relay.RelayConn != nil {
		relay.RelayConn.Close()
		relay.RelayConn = nil
	}
}
