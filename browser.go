package gosurf

import (
	"net/http"
	"net/http/httptest"
	"time"

	"h12.me/errors"
	"h12.me/mitm"
)

var (
	ErrJSRedirectionTimeout = errors.New("JS redirection timeout")
)

type Browser struct {
	Timeout time.Duration
	certs   *mitm.CertPool
	quit    chan bool
}

func NewBrowser(timeout time.Duration, certs *mitm.CertPool) *Browser {
	return &Browser{
		Timeout: timeout,
		certs:   certs,
		quit:    make(chan bool),
	}
}

func (t *Browser) Browse(req *http.Request, serve http.HandlerFunc) error {
	if t.Timeout == 0 {
		t.Timeout = 10 * time.Second
	}
	proxy := newProxy(t.Timeout, t.certs, serve)
	defer proxy.Close()

	surf, err := startSurf(req.URL.String(), proxy.URL())
	if err != nil {
		return nil
	}
	defer surf.Close()

	select {
	case <-t.quit:
	case <-time.After(t.Timeout):
		return ErrJSRedirectionTimeout
	case err := <-proxy.ErrChan():
		return err
	case err := <-errChan(surf.Wait):
		return err
	}
	return nil
}

func (b *Browser) Close() error {
	b.quit <- true
	return nil
}

type fakeProxy struct {
	serveHTTP http.HandlerFunc
	certs     *mitm.CertPool
	timeout   time.Duration
	proxy     *httptest.Server
	errChan   chan error
}

func newProxy(timeout time.Duration, certs *mitm.CertPool, serveHTTP http.HandlerFunc) *fakeProxy {
	fp := &fakeProxy{
		certs:     certs,
		timeout:   timeout,
		serveHTTP: serveHTTP,
		errChan:   make(chan error),
	}

	fp.proxy = httptest.NewServer(http.HandlerFunc(fp.serve))
	return fp
}

func (p *fakeProxy) URL() string {
	return p.proxy.URL
}

func (p *fakeProxy) ErrChan() <-chan error {
	return p.errChan
}

func (p *fakeProxy) setError(err error) {
	select {
	case p.errChan <- err:
	default:
	}
}

func (p *fakeProxy) Close() error {
	p.proxy.Close() // make should all serve goroutines have exited
	return nil
}

func (p *fakeProxy) serve(w http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		p.serveHTTP(w, req)
	} else if req.Method == "CONNECT" {
		err := p.certs.ServeHTTPS(w, req, p.serveHTTP)
		if err != nil {
			p.setError(errors.Wrap(err))
		}
	}
}
