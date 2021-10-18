package stuntest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/pion/logging"
	"github.com/xylophone21/go-turn-test/statistics"
)

var basicStunUrl string

func init() {
	basicStunUrl = os.Getenv("stunUrl")
}

func makeStunRequestST(StunServerAddr string) *StunRequestST {
	ctx, _ := context.WithTimeout(context.Background(), time.Second*15)

	f := logging.DefaultLoggerFactory{
		DefaultLogLevel: logging.LogLevelTrace,
	}
	log := f.NewLogger("stun-test")

	var req StunRequestST
	req.Ctx = ctx
	req.Log = log
	req.ChanId = 888
	req.PackageWait = time.Millisecond * 100
	req.StunServerAddr = StunServerAddr
	req.Ch = make(chan statistics.RequestResults)

	log.Infof("StunServerAddr=%s", StunServerAddr)

	go func() {
		for ret := range req.Ch {
			if ret.ErrCode != 0 {
				req.Log.Infof("get:%v", ret.ErrCode)
			} else {
				req.Log.Infof("IsSent=%v Latency=%v", ret.IsSent, ret.Latency)
			}
		}

	}()

	return &req
}

func TestBasic(t *testing.T) {
	req := makeStunRequestST(basicStunUrl)
	StunRequest(req)
}
