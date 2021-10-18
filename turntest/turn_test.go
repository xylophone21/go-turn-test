package turntest

import (
	"context"
	"testing"
	"time"

	"github.com/pion/logging"
	"github.com/xylophone21/go-turn-test/testdata"
)

func makeTrunRequestST(StunServerAddr string, TurnServerAddr string, Username string, Password string) *TrunRequestST {
	ctx, _ := context.WithTimeout(context.Background(), time.Second*15)

	f := logging.DefaultLoggerFactory{
		DefaultLogLevel: logging.LogLevelTrace,
	}
	log := f.NewLogger("turn-test")

	var req TrunRequestST
	req.Ctx = ctx
	req.Log = log
	req.ChanId = 0x1234567890
	req.PackageSize = 1024
	req.PackageWait = time.Millisecond * 1000
	req.StunServerAddr = StunServerAddr
	req.TurnServerAddr = TurnServerAddr
	req.Username = Username
	req.Password = Password

	log.Infof("TurnServerAddr=%s", TurnServerAddr)

	return &req
}

func TestBasic(t *testing.T) {
	req := makeTrunRequestST(testdata.BasicStunUrl, testdata.BasicTurnUrl, testdata.BasicTurnUsername, testdata.BasicTurnPassword)
	TrunRequest(req)
}

func TestAws(t *testing.T) {
	awsreq := &RequestBody{
		DeviceId: testdata.AwsDeviceId,
		Token:    testdata.AwsToken,
	}
	ret, err := AllocAwsTurns(awsreq)
	if err != nil {
		t.Fail()
	}

	req := makeTrunRequestST(ret.StunServerAddr, ret.TurnServerAddrs[0].TurnServerAddr, ret.TurnServerAddrs[0].Username, ret.TurnServerAddrs[0].Password)
	TrunRequest(req)
}

func Test2CloudBasic(t *testing.T) {
	req := makeTrunRequestST(testdata.BasicStunUrl, testdata.BasicTurnUrl, testdata.BasicTurnUsername, testdata.BasicTurnPassword)
	TrunRequest2Cloud(req)
}

func Test2CloudAws(t *testing.T) {
	awsreq := &RequestBody{
		DeviceId: testdata.AwsDeviceId,
		Token:    testdata.AwsToken,
	}
	ret, err := AllocAwsTurns(awsreq)
	if err != nil {
		t.Fail()
	}

	req := makeTrunRequestST(ret.StunServerAddr, ret.TurnServerAddrs[0].TurnServerAddr, ret.TurnServerAddrs[0].Username, ret.TurnServerAddrs[0].Password)
	TrunRequest2Cloud(req)
}
