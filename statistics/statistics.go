// Package statistics 统计数据
package statistics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pion/logging"
)

var (
	exportStatisticsTime = 5 * time.Second
)

type RequestResults struct {
	ChanID  uint64
	Time    time.Time
	ErrCode int  // 0 means success, or failed
	IsSent  bool // sent or receive
	Bytes   uint64
	Latency time.Duration // only for receive
}

type StatisticsRequestST struct {
	Ctx                  context.Context
	Log                  logging.LeveledLogger
	ChanCount            uint64
	Ch                   chan RequestResults
	ExportStatisticsTime time.Duration
}

type statisticsChan struct {
	FirstTime    time.Time
	LastTime     time.Time
	RecvBytes    uint64
	SentBytes    uint64
	SentCount    int
	RecvCount    int
	ErrCount     int
	LastSuccess  bool
	LatencyCount int           // How many time get latency
	LatencyTotal time.Duration // total latency
}

type statisticsClient struct {
	lock            sync.Mutex
	log             logging.LeveledLogger
	chanCount       uint64
	successCount    int
	maxSuccessCount int
	chans           map[uint64]*statisticsChan
}

func ReceivingResults(req *StatisticsRequestST) error {
	if req == nil {
		err := fmt.Errorf("[StartReceivingResults]req nil")
		return err
	}

	if req.Ctx == nil || req.Log == nil || req.ChanCount <= 0 || req.Ch == nil {
		err := fmt.Errorf("[StartReceivingResults]Paramters error")
		return err
	}

	if req.ExportStatisticsTime <= 0 {
		req.ExportStatisticsTime = exportStatisticsTime
	}

	client := &statisticsClient{
		lock:      sync.Mutex{},
		log:       req.Log,
		chanCount: req.ChanCount,
		chans:     make(map[uint64]*statisticsChan),
	}

	for {
		select {
		case ret := <-req.Ch:
			client.AddResult(&ret)

		case <-time.After(req.ExportStatisticsTime):
			client.LogDetails()

		case <-req.Ctx.Done():
			client.LogDetails()
			client.LogSummary()
			return nil
		}
	}
}

func (c *statisticsClient) AddResult(result *RequestResults) {
	if result == nil {
		c.log.Debugf("AddResult nil result")
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if result.ChanID > c.chanCount {
		c.log.Debugf("AddResult ChanID(%d) > chanCount", result.ChanID, c.chanCount)
		return
	}

	if result.Time.IsZero() {
		c.log.Debugf("AddResult Time nil")
		return
	}

	chanClient, ok := c.chans[result.ChanID]
	if !ok {
		chanClient = &statisticsChan{
			FirstTime:   result.Time,
			LastSuccess: false,
		}

		c.chans[result.ChanID] = chanClient
	}

	chanClient.LastTime = result.Time

	if result.ErrCode != 0 {
		chanClient.ErrCount++

		if chanClient.LastSuccess {
			c.successCount--
		}
		chanClient.LastSuccess = false
		return
	}

	c.log.Tracef("AddResult-%v success isSent=%v Bytes=%v", result.ChanID, result.IsSent, result.Bytes)

	if !chanClient.LastSuccess {
		c.successCount++
	}
	chanClient.LastSuccess = true

	if c.successCount > c.maxSuccessCount {
		c.maxSuccessCount = c.successCount
	}

	if result.IsSent {
		chanClient.SentCount++
		chanClient.SentBytes += result.Bytes
	} else {
		chanClient.RecvCount++
		chanClient.RecvBytes += result.Bytes

		if result.Latency > 0 {
			chanClient.LatencyCount++
			chanClient.LatencyTotal += result.Latency
		}
	}
}

func (c *statisticsClient) LogSummary() {
	c.lock.Lock()
	defer c.lock.Unlock()

	gotChanCount := 0
	successedChanCount := 0

	recvCount := 0
	recvBytes := uint64(0)

	sentCount := 0
	sentBytes := uint64(0)

	failedCount := 0

	byteRecv := uint64(0)
	timeEscape := float64(0)

	latencyTotal := time.Duration(0)
	latencyCount := 0

	for _, chanClient := range c.chans {
		gotChanCount++

		if chanClient.RecvCount > 0 {
			successedChanCount++

			recvCount += chanClient.RecvCount
			recvBytes += chanClient.RecvBytes
		}

		if chanClient.SentCount > 0 {
			sentCount += chanClient.SentCount
			sentBytes += chanClient.SentBytes
		}

		d := chanClient.LastTime.Sub(chanClient.FirstTime).Seconds()
		if chanClient.RecvBytes > 0 && d > 0 {
			byteRecv += chanClient.RecvBytes
			timeEscape += d
		}

		if chanClient.ErrCount > 0 {
			failedCount += chanClient.ErrCount
		}

		if chanClient.LatencyTotal > 0 && chanClient.LatencyCount > 0 {
			latencyTotal += chanClient.LatencyTotal
			latencyCount += chanClient.LatencyCount
		}
	}

	loss := float32(0)
	if sentCount > 0 {
		loss = 100 - float32(recvCount)/float32(sentCount)*100
	}

	kps := 0
	if timeEscape != 0 {
		kps = int(8 * float64(byteRecv) / timeEscape / 1024)
	}
	latency := int(float64(latencyTotal.Milliseconds()) * float64(1) / float64(latencyCount))

	c.log.Infof("----statistics summary----")
	c.log.Infof("ChanCount:%v", c.chanCount)
	c.log.Infof("Got ChanCount:%v", gotChanCount)
	c.log.Infof("Once Successed ChanCount:%v", successedChanCount)
	c.log.Infof("Max Successed ChanCount:%v", c.maxSuccessCount)
	c.log.Infof("Sent Count:%v", sentCount)
	c.log.Infof("Sent Bytes(KB):%v", sentBytes/1024)
	c.log.Infof("Recv Count:%v", recvCount)
	c.log.Infof("Recv Bytes(KB):%v", recvBytes/1024)
	c.log.Infof("AVG Recv(kbps):%v", kps)
	c.log.Infof("Loss:%.2v%%", loss)
	c.log.Infof("Failed Count:%v", failedCount)
	c.log.Infof("Avg Latency:%v", latency)
}

func (c *statisticsClient) LogDetails() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.log.Infof("%6s│%6s│%15s│%6s│%15s|%6s|%6s|%6s|%6s",
		"chanid", "Sent", "SentBytes(K)", "Rece", "RecvBytes(K)", "Kbps", "Loss", "Errors", "Latency")

	for chanid := uint64(0); chanid < c.chanCount; chanid++ {
		chanClient, ok := c.chans[chanid]
		if ok {
			latency := 0
			if chanClient.LatencyCount > 0 {
				latency = int(float64(chanClient.LatencyTotal.Milliseconds()) * float64(1) / float64(chanClient.LatencyCount))
			}

			since := chanClient.LastTime.Sub(chanClient.FirstTime).Seconds()
			kps := 0
			if since != 0 {
				kps = int(8 * float64(chanClient.RecvBytes) / since / 1024)
			}

			loss := float32(0)
			if chanClient.SentCount > 0 {
				loss = 100 - float32(chanClient.RecvCount)/float32(chanClient.SentCount)*100
			}

			c.log.Infof("%6d│%6d│%15d│%6d│%15d|%6d|%5.1f%%|%6d|%6d",
				chanid, chanClient.SentCount, chanClient.SentBytes/1024, chanClient.RecvCount, chanClient.RecvBytes/1024, kps, loss, chanClient.ErrCount, latency)
		}
	}
	c.log.Info("")
}
