package turntest

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	apiurl     = "https://uat.web-rtc.ipcuat.tcljd.cn/v1/call"
	awsStunUrl = "stun.kinesisvideo.cn-north-1.amazonaws.com.cn:443"
)

var awsDeviceId string
var awsToken string

type RequestBody struct {
	DeviceId string `json:"deviceId"`
	Token    string `json:"token"`
}

type server struct {
	Urls     []string `json:"urls"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	Expired  int64    `json:"expired"`
}

type responseBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		SessionId        string   `json:"sessionId"`
		AppIceServers    []server `json:"AppIceServers"`
		DeviceIceServers []server `json:"DeviceIceServers"`
	} `json:"data"`
}

type AwsTurnServers struct {
	TurnServerAddr string
	Username       string
	Password       string
	Expired        time.Time
}

type AwsTurnsServers struct {
	StunServerAddr  string
	TurnServerAddrs []AwsTurnServers
}

func init() {
	awsDeviceId = os.Getenv("deviceId")
	awsToken = os.Getenv("token")
}

func AllocAwsTurns(request *RequestBody) (*AwsTurnsServers, error) {
	requestBody := new(bytes.Buffer)
	json.NewEncoder(requestBody).Encode(request)

	req, err := http.NewRequest("POST", apiurl, requestBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var respBody responseBody
	err = json.Unmarshal(body, &respBody)
	if err != nil {
		return nil, err
	}

	ret := AwsTurnsServers{}

	for i := 0; i < len(respBody.Data.AppIceServers); i++ {
		for j := 0; j < len(respBody.Data.AppIceServers[i].Urls); j++ {
			urlStr := respBody.Data.AppIceServers[i].Urls[j]
			u, _ := url.Parse(urlStr)
			if u.Scheme == "turn" {
				turn := AwsTurnServers{
					TurnServerAddr: u.Opaque,
					Username:       respBody.Data.AppIceServers[i].Username,
					Password:       respBody.Data.AppIceServers[i].Password,
					Expired:        time.Unix(respBody.Data.AppIceServers[i].Expired, 0),
				}
				ret.TurnServerAddrs = append(ret.TurnServerAddrs, turn)

			} else if u.Scheme == "stun" {
				//todo, api issue
				ret.StunServerAddr = awsStunUrl
			}
		}
	}

	return &ret, nil
}
