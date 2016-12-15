package gosurf

import (
	"compress/gzip"
	"io"
	"net/http"
)

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",      // canonicalized version of "TE"
	"Trailer", // not Trailers per URL above; http://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",
}

func RoundTrip(roundTripper http.RoundTripper, w http.ResponseWriter, req *http.Request) error {
	for _, h := range hopHeaders {
		req.Header.Del(h)
	}
	req.RequestURI = ""
	resp, err := roundTripper.RoundTrip(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	GunzipResponse(resp)
	return WriteResponse(w, resp)
}

func WriteResponse(w http.ResponseWriter, resp *http.Response) error {
	w.WriteHeader(resp.StatusCode)
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	_, err := io.Copy(w, resp.Body)
	return err
}

type gzipReadCloser struct {
	rc io.ReadCloser
	*gzip.Reader
}

func newGzipReadCloser(rc io.ReadCloser) (*gzipReadCloser, error) {
	reader, err := gzip.NewReader(rc)
	if err != nil {
		return nil, err
	}
	return &gzipReadCloser{
		rc:     rc,
		Reader: reader,
	}, nil
}

func (r *gzipReadCloser) Close() error {
	if err := r.rc.Close(); err != nil {
		return err
	}
	return r.Reader.Close()
}

// Gzip decode the request body with header("Content-Encoding":"gzip")
func GunzipResponse(resp *http.Response) error {
	if resp.Header.Get("Content-Encoding") == "gzip" {
		rc, err := newGzipReadCloser(resp.Body)
		if err != nil {
			return err
		}
		resp.Body = rc
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
		resp.ContentLength = 0
	}
	return nil
}
