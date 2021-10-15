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

func doTurnRequest(StunServerAddr string, TurnServerAddr string, Username string, Password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()

	f := logging.DefaultLoggerFactory{
		DefaultLogLevel: logging.LogLevelTrace,
	}
	log := f.NewLogger("turn-test")

	var req TrunRequestST
	req.Ctx = ctx
	req.Log = log
	req.ChanId = 0x1234567890
	req.PackageSize = 1024
	req.PackageWaitMs = time.Millisecond * 1000
	req.StunServerAddr = StunServerAddr
	req.TurnServerAddr = TurnServerAddr
	req.Username = Username
	req.Password = Password

	log.Infof("TurnServerAddr=%s", TurnServerAddr)

	return TrunRequest(&req)
}

func TestBasic(t *testing.T) {

	err := doTurnRequest("", "", "", "")

	if err != nil {
		t.FailNow()
	}
}

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

func TestAws(t *testing.T) {
	request := RequestBody{
		DeviceId: "",
		Token:    "",
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
	for i := 0; i < len(respBody.Data.AppIceServers); i++ {
		for j := 0; j < len(respBody.Data.AppIceServers[i].Urls); j++ {
			urlStr := respBody.Data.AppIceServers[i].Urls[j]
			u, _ := url.Parse(urlStr)
			if u.Scheme == "turn" {
				err := doTurnRequest("stun.kinesisvideo.cn-north-1.amazonaws.com.cn:443", u.Opaque, respBody.Data.AppIceServers[i].Username, respBody.Data.AppIceServers[i].Password)
				if err != nil {
					t.FailNow()
				}
				break
			}

		}
	}

}
