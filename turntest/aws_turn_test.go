package turntest

import (
	"fmt"
	"testing"
	"time"
)

func TestAlloc(t *testing.T) {
	ret, err := AllocAwsTurns()
	if err != nil {
		t.Fail()
	}

	if ret.StunServerAddr == "" {
		t.Fail()
	}

	if len(ret.TurnServerAddrs) <= 0 {
		t.Fail()
	}

	for i := 0; i < len(ret.TurnServerAddrs); i++ {
		if ret.TurnServerAddrs[i].TurnServerAddr == "" {
			t.Fail()
		}

		if ret.TurnServerAddrs[i].Username == "" {
			t.Fail()
		}

		if ret.TurnServerAddrs[i].Password == "" {
			t.Fail()
		}

		if ret.TurnServerAddrs[i].Expired.Before(time.Now()) {
			t.Fail()
		}
	}

	fmt.Print(ret)
}
