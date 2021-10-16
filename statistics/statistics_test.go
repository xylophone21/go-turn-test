package statistics

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pion/logging"
)

func TestBasic(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	ctx, canceled := context.WithTimeout(context.Background(), time.Second*15)

	f := logging.DefaultLoggerFactory{
		DefaultLogLevel: logging.LogLevelTrace,
	}
	log := f.NewLogger("statistics-test")

	ch := make(chan RequestResults, 1000)

	req := &StatisticsRequestST{
		Ctx:       ctx,
		Log:       log,
		ChanCount: 2,
		Ch:        ch,
	}

	go func() {
		ReceivingResults(req)
		wg.Done()
	}()

	result := RequestResults{
		ChanID:  0,
		Time:    time.Now(),
		ErrCode: 0,
		IsSent:  true,
		Bytes:   1024,
		Latency: time.Millisecond * 10,
	}

	//send 1st
	ch <- result

	//recv 1st
	result.IsSent = false
	result.Time = result.Time.Add(time.Second)
	ch <- result

	//send 2nd
	result.IsSent = true
	result.Time = result.Time.Add(time.Second)
	ch <- result

	//recv 2nd
	result.IsSent = false
	result.Time = result.Time.Add(time.Second)
	result.Latency = time.Millisecond * 5
	ch <- result

	//send 3rd
	result.IsSent = true
	result.Time = result.Time.Add(time.Second)
	ch <- result

	//recv 3rd
	result.IsSent = false
	result.Time = result.Time.Add(time.Second)
	result.Latency = time.Millisecond * 15
	ch <- result

	result.ChanID = 1

	//send 1st
	result.IsSent = true
	result.Time = result.Time.Add(time.Second)
	ch <- result

	//lost recv 1st

	//send 2nd
	result.IsSent = true
	result.Time = result.Time.Add(time.Second)
	ch <- result

	//recv 2nd error
	result.IsSent = false
	result.ErrCode = 1
	result.Time = result.Time.Add(time.Second)
	ch <- result

	wg.Wait()
	canceled()
}
