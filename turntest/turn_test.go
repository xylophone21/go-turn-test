package turntest

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/pion/logging"
)

const (
	basicTurnUrl      = "hellohui.space:3478"
	basicTurnUsername = ""
	basicTurnPassword = ""
	awsDeviceId       = ""
	awsToken          = ""
	awsStunUrl        = "stun.kinesisvideo.cn-north-1.amazonaws.com.cn:443"
)

type RequestBody struct {
	DeviceId string `json:"deviceId"`
	Token    string `json:"token"`
}

type Server struct {
	Urls     []string `json:"urls"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	Expired  int      `json:"expired"`
}

type ResponseBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		SessionId        string   `json:"sessionId"`
		AppIceServers    []Server `json:"AppIceServers"`
		DeviceIceServers []Server `json:"DeviceIceServers"`
	} `json:"data"`
}

func makeTrunRequestST(StunServerAddr string, TurnServerAddr string, Username string, Password string, PublicIPTst bool) *TrunRequestST {
	ctx, _ := context.WithTimeout(context.Background(), time.Minute*1)

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
	req.PublicIPTst = PublicIPTst

	log.Infof("TurnServerAddr=%s", TurnServerAddr)

	return &req
}

func allocAwsTurn() *ResponseBody {
	request := RequestBody{
		DeviceId: awsDeviceId,
		Token:    awsToken,
	}
	requestBody := new(bytes.Buffer)
	json.NewEncoder(requestBody).Encode(request)

	apiurl := "https://uat.web-rtc.ipcuat.tcljd.cn/v1/call"
	req, _ := http.NewRequest("POST", apiurl, requestBody)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, _ := client.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)
	var respBody ResponseBody
	json.Unmarshal(body, &respBody)

	return &respBody
}

func TestBasic(t *testing.T) {
	req := makeTrunRequestST("", basicTurnUrl, basicTurnUsername, basicTurnPassword, false)
	TrunRequest(req)
}

func TestAws(t *testing.T) {
	respBody := allocAwsTurn()

	for i := 0; i < len(respBody.Data.AppIceServers); i++ {
		for j := 0; j < len(respBody.Data.AppIceServers[i].Urls); j++ {
			urlStr := respBody.Data.AppIceServers[i].Urls[j]
			u, _ := url.Parse(urlStr)
			if u.Scheme == "turn" {
				req := makeTrunRequestST(awsStunUrl, u.Opaque, respBody.Data.AppIceServers[i].Username, respBody.Data.AppIceServers[i].Password, true)
				TrunRequest(req)
				break
			}
		}
	}
}

func Test2CloudBasic(t *testing.T) {
	req := makeTrunRequestST("", basicTurnUrl, basicTurnUsername, basicTurnPassword, false)
	TrunRequest2Cloud(req)
}

func Test2CloudAws(t *testing.T) {
	respBody := allocAwsTurn()

	for i := 0; i < len(respBody.Data.AppIceServers); i++ {
		for j := 0; j < len(respBody.Data.AppIceServers[i].Urls); j++ {
			urlStr := respBody.Data.AppIceServers[i].Urls[j]
			u, _ := url.Parse(urlStr)
			if u.Scheme == "turn" {
				req := makeTrunRequestST(awsStunUrl, u.Opaque, respBody.Data.AppIceServers[i].Username, respBody.Data.AppIceServers[i].Password, true)
				TrunRequest2Cloud(req)
				break
			}
		}
	}
}
