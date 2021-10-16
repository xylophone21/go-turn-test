/**
* Package statistics
*
* User: link1st
* Date: 2020/9/28
* Time: 14:02
 */
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

	ctx, _ := context.WithTimeout(context.Background(), time.Second*15)

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
		ChanID:        0,
		ReceivedBytes: 1024,
		IsSucceed:     true,
		Latency:       time.Millisecond * 10,
		Time:          time.Now(),
		InitRet:       true,
	}
	ch <- result

	result.ChanID = 1
	ch <- result

	result.InitRet = false
	result.ChanID = 0

	time.Sleep(time.Second * 1)
	result.Latency = time.Millisecond * 5
	result.Time = result.Time.Add(time.Second)
	ch <- result

	result.Latency = time.Millisecond * 15
	result.Time = result.Time.Add(time.Second)
	ch <- result

	result.ChanID = 1
	result.IsSucceed = false
	result.Time = result.Time.Add(time.Second)
	ch <- result

	wg.Wait()
}
