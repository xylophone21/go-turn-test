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
	ChanID        uint64
	Time          time.Time
	InitRet       bool // first result to init statistics, ingore all result in it, to get right kps
	ReceivedBytes uint64
	IsSucceed     bool
	Latency       time.Duration
}

type StatisticsRequestST struct {
	Ctx                  context.Context
	Log                  logging.LeveledLogger
	ChanCount            uint64
	Ch                   chan RequestResults
	ExportStatisticsTime time.Duration
}

type statisticsChan struct {
	FirstTime     time.Time
	LastTime      time.Time
	ReceivedBytes uint64
	LatencyCount  int           // How many time get latency
	LatencyTotal  time.Duration // total latency
	FailedCount   int
	SuccessCount  int
	LastSuccess   bool
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

func (c *statisticsClient) AddResult(restult *RequestResults) {
	if restult == nil {
		c.log.Debugf("AddResult nil result")
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if restult.ChanID > c.chanCount {
		c.log.Debugf("AddResult ChanID(%d) > chanCount", restult.ChanID, c.chanCount)
		return
	}

	if restult.Time.IsZero() {
		c.log.Debugf("AddResult Time nil")
		return
	}

	chanClient, ok := c.chans[restult.ChanID]
	if !ok {
		chanClient = &statisticsChan{
			FirstTime:   restult.Time,
			LastSuccess: false,
		}

		c.chans[restult.ChanID] = chanClient
	}

	chanClient.LastTime = restult.Time

	if restult.InitRet {
		return
	}

	if restult.IsSucceed {
		chanClient.SuccessCount++

		if !chanClient.LastSuccess {
			c.successCount++
		}
		chanClient.LastSuccess = true
	} else {
		chanClient.FailedCount++

		if chanClient.LastSuccess {
			c.successCount--
		}
		chanClient.LastSuccess = false
	}

	if c.successCount > c.maxSuccessCount {
		c.maxSuccessCount = c.successCount
	}

	if restult.ReceivedBytes > 0 {
		chanClient.ReceivedBytes += restult.ReceivedBytes
	}

	if restult.Latency > 0 {
		chanClient.LatencyCount++
		chanClient.LatencyTotal += restult.Latency
	}
}

func (c *statisticsClient) LogSummary() {
	c.lock.Lock()
	defer c.lock.Unlock()

	gotChanCount := 0
	successedChanCount := 0
	successCount := 0
	failedCount := 0
	byteRecv := uint64(0)
	timeEscape := float64(0)

	latencyTotal := time.Duration(0)
	latencyCount := 0

	for _, chanClient := range c.chans {
		gotChanCount++

		if chanClient.SuccessCount > 0 {
			successedChanCount++

			successCount += chanClient.SuccessCount
		}

		if chanClient.FailedCount > 0 {
			failedCount += chanClient.FailedCount
		}

		d := chanClient.LastTime.Sub(chanClient.FirstTime).Seconds()
		if chanClient.ReceivedBytes > 0 && d > 0 {
			byteRecv += chanClient.ReceivedBytes
			timeEscape += d
		}

		if chanClient.LatencyTotal > 0 && chanClient.LatencyCount > 0 {
			latencyTotal += chanClient.LatencyTotal
			latencyCount += chanClient.LatencyCount
		}
	}

	kps := 0
	if timeEscape != 0 {
		kps = int(8 * float64(byteRecv) / timeEscape / 1024)
	}
	latency := int(float64(latencyTotal.Milliseconds()) * float64(1) / float64(latencyCount))

	c.log.Infof("----statistics summary----")
	c.log.Infof("ChanCount:%v", c.chanCount)
	c.log.Infof("Got ChanCount:%v", gotChanCount)
	c.log.Infof("Successed ChanCount:%v", successedChanCount)
	c.log.Infof("Max Concurrency ChanCount:%v", c.maxSuccessCount)
	c.log.Infof("Success turn Count:%v", successCount)
	c.log.Infof("Failed turn ChanCount:%v", failedCount)
	c.log.Infof("Total byte Recved:%vK", byteRecv/1024)
	c.log.Infof("AVG kbps:%v", kps)
	c.log.Infof("Avg Latency:%v", latency)
}

func (c *statisticsClient) LogDetails() {
	c.lock.Lock()
	defer c.lock.Unlock()

	header := fmt.Sprintf("%10s│%10s│%10s│%15s│%10s|%10s",
		"chanid", "Success", "Failed", "BytesReced(K)", "Kbps", "Latency")
	c.log.Info(header)

	for chanid := uint64(0); chanid < c.chanCount; chanid++ {
		chanClient, ok := c.chans[chanid]
		if ok {
			latency := int(float64(chanClient.LatencyTotal.Milliseconds()) * float64(1) / float64(chanClient.LatencyCount))

			since := chanClient.LastTime.Sub(chanClient.FirstTime).Seconds()
			kps := 0
			if since != 0 {
				kps = int(8 * float64(chanClient.ReceivedBytes) / since / 1024)
			}

			result := fmt.Sprintf("%10d│%10d│%10d│%15d|%10d│%10d",
				chanid, chanClient.SuccessCount, chanClient.FailedCount, chanClient.ReceivedBytes/1024, kps, latency)

			c.log.Info(result)
		}
	}
}
