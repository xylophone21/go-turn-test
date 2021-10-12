package turntest

import (
	"context"
	"testing"
	"time"

	"github.com/pion/logging"
)

func TestBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()

	f := logging.DefaultLoggerFactory{
		DefaultLogLevel: logging.LogLevelTrace,
	}
	log := f.NewLogger("turn-test")

	var req TrunRequestST
	req.Ctx = ctx
	req.Log = log
	req.ChanId = 0x1234567890
	req.PackageSize = 1024
	req.PackageWaitMs = time.Millisecond * 1000
	req.StunServerAddr = "todo"
	req.TurnServerAddr = "todo"
	req.Username = "todo"
	req.Password = "todo"

	err := TrunRequest(&req)
	if err != nil {
		t.FailNow()
	}
}
