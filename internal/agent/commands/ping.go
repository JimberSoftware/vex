package commands

import "github.com/jimbersoftware/vex/internal/vmp"

func ping(_ *vmp.PingRequest) *vmp.Response {
	return &vmp.Response{
		Result: &vmp.Response_Ping{Ping: &vmp.PingResponse{}},
	}
}
