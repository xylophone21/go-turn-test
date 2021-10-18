package testdata

import "os"

var BasicStunUrl string
var BasicTurnUrl string
var BasicTurnUsername string
var BasicTurnPassword string
var AwsDeviceId string
var AwsToken string

func init() {
	BasicStunUrl = os.Getenv("stunUrl")
	BasicTurnUrl = os.Getenv("turnUrl")
	BasicTurnUsername = os.Getenv("turnUsername")
	BasicTurnPassword = os.Getenv("turnPassword")
	AwsDeviceId = os.Getenv("deviceId")
	AwsToken = os.Getenv("token")
}
