package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pion/logging"
	"github.com/xylophone21/go-turn-test/statistics"
	"github.com/xylophone21/go-turn-test/testdata"
	"github.com/xylophone21/go-turn-test/turntest"
)

type monitorStatisticsClient struct {
	lock         sync.Mutex
	log          logging.LeveledLogger
	ch           chan statistics.RequestResults
	chanId       uint64
	firstTime    time.Time
	lastTime     time.Time
	recvBytes    uint64
	sentBytes    uint64
	sentCount    int
	recvCount    int
	errCount     int
	latencyCount int           // How many time get latency
	latencyTotal time.Duration // total latency
}

func newMonitorStatistics(log logging.LeveledLogger) *monitorStatisticsClient {
	ch := make(chan statistics.RequestResults, 1000)

	client := &monitorStatisticsClient{
		lock: sync.Mutex{},
		log:  log,
		ch:   ch,
	}

	go func() {
		for {
			select {
			case ret := <-ch:
				client.addResult(&ret)
			}
		}
	}()

	return client
}

func (c *monitorStatisticsClient) Reset(chanid uint64) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.chanId = chanid
	c.firstTime = time.Unix(0, 0)
	c.lastTime = time.Unix(0, 0)
	c.recvBytes = 0
	c.recvCount = 0
	c.sentCount = 0
	c.recvCount = 0
	c.errCount = 0
	c.latencyCount = 0
	c.latencyTotal = time.Duration(0)

	return nil
}

func (c *monitorStatisticsClient) GetCh() chan statistics.RequestResults {
	return c.ch
}

func (c *monitorStatisticsClient) GetResult(prefix string) string {
	c.lock.Lock()
	defer c.lock.Unlock()

	loss := float32(0)
	if c.sentCount > 0 {
		loss = 100 - float32(c.recvCount)/float32(c.sentCount)*100
	}

	timeEscape := c.lastTime.Sub(c.firstTime).Seconds()
	kps := 0
	if timeEscape != 0 {
		kps = int(8 * float64(c.recvBytes) / timeEscape / 1024)
	}
	latency := int(float64(c.latencyTotal.Milliseconds()) * float64(1) / float64(c.latencyCount))

	result := "success"
	if c.recvCount <= 0 {
		result = "failed"
	}

	return fmt.Sprintf("%v from %v to %v(%d sec): %v, loss:%.2v%%, kbps:%v, latency:%v", prefix, c.firstTime.Format("2006-01-02 15:04:05"), c.lastTime.Format("2006-01-02 15:04:05"), int(timeEscape), result, loss, kps, latency)
}

func (c *monitorStatisticsClient) addResult(result *statistics.RequestResults) {
	if result == nil {
		c.log.Warnf("addResult nil result")
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if result.ChanID != c.chanId {
		c.log.Warnf("addResult chanId missmatch, want %v got %v", c.chanId, result.ChanID)
		return
	}

	if result.Time.IsZero() {
		c.log.Warnf("addResult Time IsZero")
		return
	}

	if c.firstTime.Unix() == 0 {
		c.firstTime = result.Time
	}
	c.lastTime = result.Time

	if result.ErrCode != 0 {
		c.errCount++
		return
	}

	c.log.Tracef("addResult-%v success isSent=%v Bytes=%v", result.ChanID, result.IsSent, result.Bytes)

	if result.IsSent {
		c.sentCount++
		c.sentBytes += result.Bytes
	} else {
		c.recvCount++
		c.recvBytes += result.Bytes

		if result.Latency > 0 {
			c.latencyCount++
			c.latencyTotal += result.Latency
		}
	}
}

func doMonitor(log logging.LeveledLogger, chanid uint64, ch chan statistics.RequestResults) error {
	awsReq := &turntest.RequestBody{
		DeviceId: testdata.AwsDeviceId,
		Token:    testdata.AwsToken,
	}
	awsServers, err := turntest.AllocAwsTurns(awsReq)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	turnReq := &turntest.TrunRequestST{
		Ctx:            ctx,
		Log:            log,
		ChanId:         chanid,
		PackageSize:    1024,
		PackageWait:    time.Millisecond * 7,
		StunServerAddr: awsServers.StunServerAddr,
		TurnServerAddr: awsServers.TurnServerAddrs[0].TurnServerAddr,
		Username:       awsServers.TurnServerAddrs[0].Username,
		Password:       awsServers.TurnServerAddrs[0].Password,
		Ch:             ch,
	}

	err = turntest.TrunRequest2Cloud(turnReq)
	cancel()

	return err
}

func main() {
	f := logging.DefaultLoggerFactory{
		DefaultLogLevel: logging.LogLevelError,
	}
	log := f.NewLogger("monitor")

	client := newMonitorStatistics(log)

	var chanId uint64
	for {
		client.Reset(chanId)
		ch := client.GetCh()
		doMonitor(log, chanId, ch)
		ret := client.GetResult("Calling")
		fmt.Println(ret)

		time.Sleep(time.Minute * 5)
	}

}
