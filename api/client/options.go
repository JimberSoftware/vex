package client

import "net/http"

type Option func(*Client)

func WithHTTPClient(hc *http.Client) Option {
	return func(cl *Client) {
		cl.httpClient = hc
	}
}

func WithHeader(key, value string) Option {
	return func(cl *Client) {
		cl.headers[key] = value
	}
}
