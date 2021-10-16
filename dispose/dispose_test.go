package dispose

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pion/logging"
)

var basicTurnUrl string
var basicTurnUsername string
var basicTurnPassword string

func init() {
	basicTurnUrl = os.Getenv("turnUrl")
	basicTurnUsername = os.Getenv("turnUsername")
	basicTurnPassword = os.Getenv("turnPassword")

	basicTurnUrl = "hellohui.space:3478"
	basicTurnUsername = "lihui02"
	basicTurnPassword = "passwordlh02"
}

func TestBasic(t *testing.T) {
	req := &DisposeRequestST{
		StatLogLvl:     int(logging.LogLevelTrace),
		ReqLogLvl:      int(logging.LogLevelTrace),
		Mode:           MODE_1CLOUD,
		PublicIPTst:    false,
		Source:         SOURCE_BASE,
		StunServerAddr: basicTurnUrl,
		TurnServerAddr: basicTurnUrl,
		Username:       basicTurnUsername,
		Password:       basicTurnPassword,
		Duration:       time.Second * 10,
	}

	err := Dispose(req)
	if err != nil {
		fmt.Printf("Dispose error:%v", err)
		t.Fail()
	}
}

func TestAws(t *testing.T) {
	req := &DisposeRequestST{
		StatLogLvl:  int(logging.LogLevelTrace),
		ReqLogLvl:   int(logging.LogLevelTrace),
		Mode:        MODE_1CLOUD,
		PublicIPTst: true,
		Source:      SOURCE_AWS,
	}

	err := Dispose(req)
	if err != nil {
		fmt.Printf("Dispose error:%v", err)
		t.Fail()
	}
}

func TestBase2Cloud(t *testing.T) {
	req := &DisposeRequestST{
		StatLogLvl:     int(logging.LogLevelTrace),
		ReqLogLvl:      int(logging.LogLevelTrace),
		Mode:           MODE_2CLOUD,
		PublicIPTst:    false,
		Source:         SOURCE_BASE,
		StunServerAddr: basicTurnUrl,
		TurnServerAddr: basicTurnUrl,
		Username:       basicTurnUsername,
		Password:       basicTurnPassword,
	}

	err := Dispose(req)
	if err != nil {
		fmt.Printf("Dispose error:%v", err)
		t.Fail()
	}
}

func TestAws2Cloud(t *testing.T) {
	req := &DisposeRequestST{
		StatLogLvl:  int(logging.LogLevelTrace),
		ReqLogLvl:   int(logging.LogLevelTrace),
		Mode:        MODE_2CLOUD,
		PublicIPTst: false,
		Source:      SOURCE_AWS,
	}

	err := Dispose(req)
	if err != nil {
		fmt.Printf("Dispose error:%v", err)
		t.Fail()
	}
}
