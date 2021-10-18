package stuntest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pion/logging"
	"github.com/pion/stun"
	"github.com/xylophone21/go-turn-test/statistics"
)

type StunRequestST struct {
	Ctx            context.Context
	Log            logging.LeveledLogger
	ChanId         uint64
	StunServerAddr string // STUN server address (e.g. "stun.abc.com:3478")
	Ch             chan statistics.RequestResults
}

func sendErrorRequestResults(req *StunRequestST, errCode int) {
	if req.Ch != nil {
		result := statistics.RequestResults{
			ChanID:  req.ChanId,
			Time:    time.Now(),
			ErrCode: errCode,
		}

		req.Ch <- result
	}
}

func sendSuccessRequestResults(req *StunRequestST, isSent bool, latency *time.Duration) {
	if req.Ch != nil {
		result := statistics.RequestResults{
			ChanID:  req.ChanId,
			Time:    time.Now(),
			ErrCode: 0,
			IsSent:  isSent,
			Bytes:   128, //128bytes each package, so kbps = qps
		}

		if !isSent {
			result.Latency = *latency
		}

		req.Log.Tracef("SendResult-%v isSent=%v Bytes=%v", result.ChanID, result.IsSent, result.Bytes)

		req.Ch <- result
	}
}

func doStunRequest(req *StunRequestST) {
	var wg sync.WaitGroup
	wg.Add(1)

	c, err := stun.Dial("udp", req.StunServerAddr)
	if err != nil {
		req.Log.Warnf("[doStunRequest-%d]stun.Dial error:%s", req.ChanId, err)
		sendErrorRequestResults(req, 100)
		return
	}
	defer c.Close()

	c.SetRTO(time.Second * 5)

	msg := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	var start time.Time

	handler := func(res stun.Event) {
		defer wg.Done()
		end := time.Now()

		if res.Error != nil {
			req.Log.Warnf("[doStunRequest-%d]handler error:%s", req.ChanId, err)
			sendErrorRequestResults(req, 200)
			return
		}
		var xorAddr stun.XORMappedAddress
		if getErr := xorAddr.GetFrom(res.Message); getErr != nil {
			req.Log.Warnf("[doStunRequest-%d]xorAddr.GetFrom error:%s", req.ChanId, getErr)
			sendErrorRequestResults(req, 201)
			return
		}

		if res.Message.TransactionID != msg.TransactionID {
			req.Log.Warnf("[doStunRequest-%d]TransactionID differ", req.ChanId)
			sendErrorRequestResults(req, 201)
			return
		}

		latency := end.Sub(start)
		sendSuccessRequestResults(req, false, &latency)
	}

	start = time.Now()
	if err = c.Start(msg, handler); err != nil {
		req.Log.Warnf("[doStunRequest-%d]c.Start error:%s", req.ChanId, err)
		sendErrorRequestResults(req, 101)
		return
	}
	sendSuccessRequestResults(req, true, nil)

	wg.Wait()
}

func StunRequest(req *StunRequestST) error {
	if req == nil {
		err := fmt.Errorf("[StunRequest-unkonw]req nil")
		return err
	}

	if req.Ctx == nil || req.Log == nil || req.StunServerAddr == "" {
		err := fmt.Errorf("[StunRequest-%d]Paramters error", req.ChanId)
		return err
	}

	for {
		// timeout or canceled, return
		select {
		case <-req.Ctx.Done():
			return nil

		default:
			doStunRequest(req)
		}
	}
}
