package dispose

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pion/logging"
	"github.com/xylophone21/go-turn-test/statistics"
	"github.com/xylophone21/go-turn-test/stuntest"
	"github.com/xylophone21/go-turn-test/turntest"
)

type DisposeMode int32
type DisposeSource int32
type DisposeMethod int32

const (
	MODE_1CLOUD DisposeMode   = 0
	MODE_2CLOUD DisposeMode   = 1
	SOURCE_BASE DisposeSource = 0
	SOURCE_AWS  DisposeSource = 1
	METHOD_STUN               = 0
	METHOD_TURN               = 1
)

type DisposeRequestST struct {
	ChanCount   uint64
	Method      DisposeMethod
	Duration    time.Duration
	PackageSize int32
	PackageWait time.Duration
	StatLogLvl  int
	ReqLogLvl   int

	Mode DisposeMode

	Source         DisposeSource
	StunServerAddr string // STUN server address (e.g. "stun.abc.com:3478")
	TurnServerAddr string // TURN server addrees (e.g. "turn.abc.com:3478")
	Username       string
	Password       string

	AwsDeviceId string
	AwsToken    string
}

func checkAndDefaultRequest(req *DisposeRequestST) error {
	if req == nil {
		return fmt.Errorf("req nil")
	}

	if req.Method == METHOD_TURN && req.Source == SOURCE_BASE && req.TurnServerAddr == "" {
		return fmt.Errorf("base mode without turn server")
	}

	if req.Method == METHOD_STUN && req.Source == SOURCE_BASE && req.StunServerAddr == "" {
		return fmt.Errorf("base mode without stun server")
	}

	if req.ChanCount == 0 {
		req.ChanCount = 5
	}

	if req.Duration <= 0 {
		req.Duration = time.Second * 30
	}

	if req.PackageSize <= 0 {
		req.PackageSize = 1024
	}

	if req.PackageWait <= 0 {
		req.PackageWait = time.Second
	}

	if req.StatLogLvl <= 0 {
		req.StatLogLvl = int(logging.LogLevelInfo)
	}

	if req.ReqLogLvl <= 0 {
		req.ReqLogLvl = int(logging.LogLevelWarn)
	}

	return nil
}

func Dispose(req *DisposeRequestST) error {
	err := checkAndDefaultRequest(req)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)

	statisticsFactory := logging.DefaultLoggerFactory{
		DefaultLogLevel: logging.LogLevel(req.StatLogLvl),
	}
	statisticsLog := statisticsFactory.NewLogger("statistics")

	reqFactory := logging.DefaultLoggerFactory{
		DefaultLogLevel: logging.LogLevel(req.ReqLogLvl),
	}
	reqLog := reqFactory.NewLogger("turn-request")

	ch := make(chan statistics.RequestResults, 1000)

	ctx, canceled := context.WithTimeout(context.Background(), req.Duration)

	statReq := &statistics.StatisticsRequestST{
		Ctx:       ctx,
		Log:       statisticsLog,
		ChanCount: req.ChanCount,
		Ch:        ch,
	}

	go func() {
		statistics.ReceivingResults(statReq)
		wg.Done()
	}()

	var awsTurn *turntest.AwsTurnsServers
	index := 0

	for i := uint64(0); i < req.ChanCount; i++ {
		if req.Method == METHOD_TURN {
			turnReq := &turntest.TrunRequestST{
				Ctx:         ctx,
				Log:         reqLog,
				ChanId:      i,
				PackageSize: req.PackageSize,
				PackageWait: req.PackageWait,
				Ch:          ch,
			}

			if req.Source == SOURCE_BASE {
				turnReq.StunServerAddr = req.StunServerAddr
				turnReq.TurnServerAddr = req.TurnServerAddr
				turnReq.Username = req.Username
				turnReq.Password = req.Password
			} else {
				if awsTurn == nil || index >= len(awsTurn.TurnServerAddrs) {
					var err error
					awsReq := &turntest.RequestBody{
						DeviceId: req.AwsDeviceId,
						Token:    req.AwsToken,
					}
					awsTurn, err = turntest.AllocAwsTurns(awsReq)
					if err != nil {
						reqLog.Errorf("AllocAwsTurns error:%v", err)
						canceled()
						wg.Done()
						break
					}
					index = 0
				}

				turnReq.StunServerAddr = awsTurn.StunServerAddr
				turnReq.TurnServerAddr = awsTurn.TurnServerAddrs[index].TurnServerAddr
				turnReq.Username = awsTurn.TurnServerAddrs[index].Username
				turnReq.Password = awsTurn.TurnServerAddrs[index].Password
				index++
			}

			if req.Mode == MODE_1CLOUD {
				go turntest.TrunRequest(turnReq)
			} else {
				go turntest.TrunRequest2Cloud(turnReq)
			}
		} else if req.Method == METHOD_STUN {
			stunReq := &stuntest.StunRequestST{
				Ctx:    ctx,
				Log:    reqLog,
				ChanId: i,
				Ch:     ch,
			}

			if req.Source == SOURCE_BASE {
				stunReq.StunServerAddr = req.StunServerAddr
			} else {
				if awsTurn == nil {
					var err error
					awsReq := &turntest.RequestBody{
						DeviceId: req.AwsDeviceId,
						Token:    req.AwsToken,
					}
					awsTurn, err = turntest.AllocAwsTurns(awsReq)
					if err != nil {
						reqLog.Errorf("AllocAwsTurns error:%v", err)
						canceled()
						wg.Done()
						break
					}
				}

				stunReq.StunServerAddr = awsTurn.StunServerAddr
			}

			go stuntest.StunRequest(stunReq)
		} else {
			err = fmt.Errorf("error method")
			break
		}

		time.Sleep(5 * time.Millisecond)
	}

	wg.Wait()
	canceled()

	return nil
}
