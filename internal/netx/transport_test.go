package netx

import (
	"net/http"
	"testing"
)

type idleTransport struct{ closed bool }

func (t *idleTransport) RoundTrip(*http.Request) (*http.Response, error) { panic("unexpected request") }
func (t *idleTransport) CloseIdleConnections()                           { t.closed = true }

func TestClientClosesWrappedIdleTransport(t *testing.T) {
	base := &idleTransport{}
	client := &http.Client{Transport: userAgentTransport{base}}
	client.CloseIdleConnections()
	if !base.closed {
		t.Fatal("http.Client did not close the underlying idle connections")
	}
}
