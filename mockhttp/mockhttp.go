package mockhttp

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"

	"github.com/qiniu/x/log"
)

var (
	ErrServerNotFound = errors.New("server not found")
)

// --------------------------------------------------------------------

type mockServerRequestBody struct {
	reader      io.Reader
	closeSignal bool
}

func (r *mockServerRequestBody) Read(p []byte) (int, error) {
	if r.closeSignal || r.reader == nil {
		return 0, io.EOF
	}
	return r.reader.Read(p)
}

func (r *mockServerRequestBody) Close() error {
	r.closeSignal = true
	if c, ok := r.reader.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// --------------------------------------------------------------------
// type Transport

type Transport struct {
	route      map[string]http.Handler
	remoteAddr string
}

func NewTransport() *Transport {
	return &Transport{
		route:      make(map[string]http.Handler),
		remoteAddr: "127.0.0.1:13579",
	}
}

func (p *Transport) SetRemoteAddr(remoteAddr string) *Transport {
	p.remoteAddr = remoteAddr
	return p
}

func (p *Transport) ListenAndServe(host string, h http.Handler) {
	if h == nil {
		h = http.DefaultServeMux
	}
	p.route[host] = h
}

func (p *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	h := p.route[req.URL.Host]
	if h == nil {
		log.Warn("Server not found:", req.Host, "-", req.URL.Host)
		return nil, ErrServerNotFound
	}

	cp := *req
	cp.RemoteAddr = p.remoteAddr
	cp.Body = &mockServerRequestBody{req.Body, false}
	req = &cp

	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	req.Body.Close()

	ctlen := int64(-1)
	if v := rw.Header().Get("Content-Length"); v != "" {
		ctlen, _ = strconv.ParseInt(v, 10, 64)
	}

	return &http.Response{
		Status:           "",
		StatusCode:       rw.Code,
		Header:           rw.Header(),
		Body:             ioutil.NopCloser(rw.Body),
		ContentLength:    ctlen,
		TransferEncoding: nil,
		Close:            false,
		Trailer:          nil,
		Request:          req,
	}, nil
}

// --------------------------------------------------------------------

var DefaultTransport = NewTransport()
var DefaultClient = &http.Client{Transport: DefaultTransport}

func ListenAndServe(host string, h http.Handler) {
	DefaultTransport.ListenAndServe(host, h)
}

// --------------------------------------------------------------------
