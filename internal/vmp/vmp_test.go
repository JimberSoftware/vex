package vmp_test

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/jimbersoftware/vex/internal/vmp"
)

func TestRequestRoundTrip(t *testing.T) {
	t.Parallel()

	req := &vmp.Request{
		Id:      42,
		Command: &vmp.Request_Ping{Ping: &vmp.PingRequest{}},
	}

	var buf bytes.Buffer
	if err := vmp.WriteRequest(&buf, req); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}

	got, err := vmp.ReadRequest(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("ReadRequest: %v", err)
	}
	if got.GetId() != 42 {
		t.Errorf("id: got %d, want 42", got.GetId())
	}
	if _, ok := got.GetCommand().(*vmp.Request_Ping); !ok {
		t.Error("expected ping command")
	}
}

func TestResponseRoundTrip(t *testing.T) {
	t.Parallel()

	resp := &vmp.Response{
		Id:     7,
		Result: &vmp.Response_Ping{Ping: &vmp.PingResponse{}},
	}

	var buf bytes.Buffer
	if err := vmp.WriteResponse(&buf, resp); err != nil {
		t.Fatalf("WriteResponse: %v", err)
	}

	got, err := vmp.ReadResponse(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	if got.GetId() != 7 {
		t.Errorf("id: got %d, want 7", got.GetId())
	}
	if _, ok := got.GetResult().(*vmp.Response_Ping); !ok {
		t.Error("expected ping result")
	}
}
