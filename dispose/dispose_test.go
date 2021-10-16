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
}

func TestBasic(t *testing.T) {
	req := &DisposeRequestST{
		StatLogLvl:     int(logging.LogLevelTrace),
		ReqLogLvl:      int(logging.LogLevelTrace),
		Mode:           MODE_1CLOUD,
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
		StatLogLvl: int(logging.LogLevelTrace),
		ReqLogLvl:  int(logging.LogLevelTrace),
		Mode:       MODE_1CLOUD,
		Source:     SOURCE_AWS,
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
		StatLogLvl: int(logging.LogLevelTrace),
		ReqLogLvl:  int(logging.LogLevelTrace),
		Mode:       MODE_2CLOUD,
		Source:     SOURCE_AWS,
	}

	err := Dispose(req)
	if err != nil {
		fmt.Printf("Dispose error:%v", err)
		t.Fail()
	}
}
