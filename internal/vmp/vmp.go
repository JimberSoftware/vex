package vmp

//go:generate protoc --go_out=. --go_opt=paths=source_relative vmp.proto

import (
	"bufio"
	"io"

	"google.golang.org/protobuf/encoding/protodelim"
)

func ReadRequest(r *bufio.Reader) (*Request, error) {
	req := &Request{}
	if err := protodelim.UnmarshalFrom(r, req); err != nil {
		return nil, err
	}
	return req, nil
}

func WriteRequest(w io.Writer, req *Request) error {
	_, err := protodelim.MarshalTo(w, req)
	return err
}

func ReadResponse(r *bufio.Reader) (*Response, error) {
	resp := &Response{}
	if err := protodelim.UnmarshalFrom(r, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func WriteResponse(w io.Writer, resp *Response) error {
	_, err := protodelim.MarshalTo(w, resp)
	return err
}
