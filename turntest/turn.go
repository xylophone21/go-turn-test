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
	"github.com/xylophone21/go-turn-test/statistics"
)

const (
	minPackageSize = 128
	minPackageWait = time.Microsecond * 100

	chanIdOffset  = 0
	timeLenOffset = chanIdOffset + 8
	timeOffset    = timeLenOffset + 4
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
	Ch             chan statistics.RequestResults
}

type relayClient struct {
	Conn      net.PacketConn
	Client    *turn.Client
	RelayConn net.PacketConn
}

func sendErrorRequestResults(req *TrunRequestST, errCode int) {
	if req.Ch != nil {
		result := statistics.RequestResults{
			ChanID:  req.ChanId,
			Time:    time.Now(),
			ErrCode: errCode,
		}

		req.Ch <- result
	}
}

func sendSuccessRequestResults(req *TrunRequestST, isSent bool, bytes uint64, latency *time.Duration) {
	if req.Ch != nil {
		result := statistics.RequestResults{
			ChanID:  req.ChanId,
			Time:    time.Now(),
			ErrCode: 0,
			IsSent:  isSent,
			Bytes:   bytes,
		}

		if !isSent {
			result.Latency = *latency
		}

		req.Log.Tracef("SendResult-%v isSent=%v Bytes=%v", result.ChanID, result.IsSent, result.Bytes)

		req.Ch <- result
	}
}

func requestWrap(req *TrunRequestST, doRequest func(req *TrunRequestST) error) error {
	if req == nil {
		err := fmt.Errorf("[requestWrap-unkonw]req nil")
		return err
	}

	if req.Ctx == nil || req.Log == nil || req.PackageSize < minPackageSize || req.PackageWait < minPackageWait || req.TurnServerAddr == "" {
		err := fmt.Errorf("[requestWrap-%d]Paramters error", req.ChanId)
		return err
	}

	if req.StunServerAddr == "" {
		req.StunServerAddr = req.TurnServerAddr
	}

	for {
		doRequest(req)

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

func readAndVerifyDataback(req *TrunRequestST, conn net.PacketConn, start time.Time) {
	var byteRecv uint64 = 0
	recvBuf := make([]byte, req.PackageSize+32)
	for {
		n, _, err := conn.ReadFrom(recvBuf)
		if err != nil {
			req.Log.Warnf("[readAndVerifyDataback-%d]conn.ReadFrom error:%s", req.ChanId, err)
			sendErrorRequestResults(req, 1000)
			return
		}

		if string(recvBuf[:n]) == "Hello" {
			continue
		}

		if n != int(req.PackageSize) {
			req.Log.Warnf("[readAndVerifyDataback-%d]conn.ReadFrom len error,want %d got %d", req.ChanId, req.PackageSize, n)
			sendErrorRequestResults(req, 1001)
			continue
		}

		chanId := binary.BigEndian.Uint64(recvBuf[chanIdOffset:])
		if chanId != req.ChanId {
			req.Log.Warnf("[readAndVerifyDataback-%d]chanId error:%d", req.ChanId, chanId)
			sendErrorRequestResults(req, 1002)
			continue
		}

		timeLen := int(binary.BigEndian.Uint32(recvBuf[timeLenOffset:]))

		timeStr := string(recvBuf[timeOffset : timeOffset+timeLen])
		sentAt, err := time.Parse(time.RFC3339Nano, timeStr)
		if err != nil {
			req.Log.Warnf("[readAndVerifyDataback-%d]time.Parse error:%s", req.ChanId, err)
			sendErrorRequestResults(req, 1003)
			continue
		}

		crc32Get := binary.BigEndian.Uint32(recvBuf[req.PackageSize-8:])
		crc32Sum := crc32.ChecksumIEEE(recvBuf[:req.PackageSize-8])
		if crc32Get != crc32Sum {
			req.Log.Warnf("[readAndVerifyDataback-%d]crc error, want %x got %x", req.ChanId, crc32Sum, crc32Get)
			sendErrorRequestResults(req, 1004)
			continue
		}

		byteRecv += uint64(n)
		delay := time.Since(sentAt)
		sendSuccessRequestResults(req, false, uint64(n), &delay)

		since := time.Since(start).Seconds()
		if delay.Milliseconds() > 0 {
			req.Log.Infof("[readAndVerifyDataback-%d] Recv %d kps delay=%d", req.ChanId, int(8*float64(byteRecv)/since/1024), delay.Milliseconds())
		}
	}
}

func sendData(req *TrunRequestST, conn net.PacketConn, toAddr net.Addr, start time.Time) error {
	sendBuf := make([]byte, req.PackageSize)
	rand.Read(sendBuf)

	var byteSend uint64 = 0
	for {
		select {
		case <-req.Ctx.Done():
			return nil

		default:
		}

		binary.BigEndian.PutUint64(sendBuf[chanIdOffset:], req.ChanId)

		nowStr := time.Now().Format(time.RFC3339Nano)
		binary.BigEndian.PutUint32(sendBuf[timeLenOffset:], uint32(len(nowStr)))
		copy(sendBuf[timeOffset:], []byte(nowStr))

		crc32 := crc32.ChecksumIEEE(sendBuf[:req.PackageSize-8])
		binary.BigEndian.PutUint32(sendBuf[req.PackageSize-8:], crc32)

		_, err := conn.WriteTo(sendBuf, toAddr)
		if err != nil {
			req.Log.Warnf("[sendData-%d]conn.WriteTo error:%s", req.ChanId, err)
			sendErrorRequestResults(req, 2000)
			return err
		}
		byteSend += uint64(len(sendBuf))

		sendSuccessRequestResults(req, true, uint64(len(sendBuf)), nil)

		time.Sleep(req.PackageWait)
		since := time.Since(start).Seconds()
		if since > 0 {
			req.Log.Infof("[sendData-%d] Send %d kps", req.ChanId, int(8*float64(byteSend)/since/1024))
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
func doTrunRequest(req *TrunRequestST) error {
	relay, err := allocRelayClient(req)
	if err != nil {
		sendErrorRequestResults(req, 100)
		return err
	}
	defer freeRelayClient(relay)

	// Set up sender socket (senderConn)
	var lc net.ListenConfig
	senderConn, err := lc.ListenPacket(req.Ctx, "udp4", "0.0.0.0:0")
	if err != nil {
		req.Log.Warnf("[doTrunRequest-%d]lc.ListenPacket error:%s", req.ChanId, err)
		sendErrorRequestResults(req, 101)
		return err
	}
	defer senderConn.Close()

	// Send BindingRequest to learn our external IP
	mappedAddr, err := relay.Client.SendBindingRequest()
	if err != nil {
		req.Log.Warnf("[TrunRequest-%d]client.SendBindingRequest() error:%s", req.ChanId, err)
		sendErrorRequestResults(req, 102)
		return err
	}

	// [workaround] server with pulibc ip will usually have a local ip but mapping all port to local ip
	// so use public ip and connection port
	// for standard mode, port in mappedAddr will be ingore, so changing it will make no error
	addrIp := strings.Split(mappedAddr.String(), ":")
	addrPort := strings.Split(senderConn.LocalAddr().String(), ":")
	mappedAddr, _ = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%s", addrIp[0], addrPort[1]))

	// added mappedAddr (without port) to permission list in turn server
	_, err = relay.RelayConn.WriteTo([]byte("Hello"), mappedAddr)
	if err != nil {
		req.Log.Warnf("[TrunRequest-%d]relayConn.WriteTo error:%s", req.ChanId, err)
		sendErrorRequestResults(req, 103)
		return err
	}

	timeSend := time.Now()
	go readAndVerifyDataback(req, relay.RelayConn, timeSend)

	err = sendData(req, senderConn, relay.RelayConn.LocalAddr(), timeSend)

	relay.Client.Close()
	senderConn.Close()

	return nil
}

func doTrunRequest2Cloud(req *TrunRequestST) error {
	relay1, err := allocRelayClient(req)
	if err != nil {
		sendErrorRequestResults(req, 200)
		return err
	}
	defer freeRelayClient(relay1)

	relay2, err := allocRelayClient(req)
	if err != nil {
		sendErrorRequestResults(req, 201)
		return err
	}
	defer freeRelayClient(relay2)

	// added mappedAddr (without port) to permission list in turn server
	_, err = relay1.RelayConn.WriteTo([]byte("Hello"), relay2.RelayConn.LocalAddr())
	if err != nil {
		req.Log.Warnf("[TrunRequest2Cloud-%d]relayConn.WriteTo error:%s", req.ChanId, err)
		sendErrorRequestResults(req, 202)
		return err
	}
	_, err = relay2.RelayConn.WriteTo([]byte("Hello"), relay1.RelayConn.LocalAddr())
	if err != nil {
		req.Log.Warnf("[TrunRequest2Cloud-%d]relayConn.WriteTo error:%s", req.ChanId, err)
		sendErrorRequestResults(req, 203)
		return err
	}

	timeSend := time.Now()
	go readAndVerifyDataback(req, relay2.RelayConn, timeSend)

	err = sendData(req, relay1.RelayConn, relay2.RelayConn.LocalAddr(), timeSend)
	relay1.RelayConn.Close()
	relay2.RelayConn.Close()

	return err
}

func TrunRequest(req *TrunRequestST) error {
	return requestWrap(req, doTrunRequest)
}

// PeerA <--> RelayA  <---> RelayB  <---> PeerB
func TrunRequest2Cloud(req *TrunRequestST) error {
	return requestWrap(req, doTrunRequest2Cloud)
}
