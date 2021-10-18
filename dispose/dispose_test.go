package dispose

import (
	"fmt"
	"testing"
	"time"

	"github.com/pion/logging"
	"github.com/xylophone21/go-turn-test/testdata"
)

func TestBasic(t *testing.T) {
	req := &DisposeRequestST{
		StatLogLvl:     int(logging.LogLevelTrace),
		ReqLogLvl:      int(logging.LogLevelTrace),
		Mode:           MODE_1CLOUD,
		Source:         SOURCE_BASE,
		StunServerAddr: testdata.BasicStunUrl,
		TurnServerAddr: testdata.BasicTurnUrl,
		Username:       testdata.BasicTurnUsername,
		Password:       testdata.BasicTurnPassword,
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
		StunServerAddr: testdata.BasicStunUrl,
		TurnServerAddr: testdata.BasicTurnUrl,
		Username:       testdata.BasicTurnUsername,
		Password:       testdata.BasicTurnPassword,
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
