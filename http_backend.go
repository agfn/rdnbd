package rdnbd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/pojntfx/go-nbd/pkg/backend"
	"github.com/sirupsen/logrus"
)

type httpBackend struct {
	url       string
	rangeUnit string
	client    *http.Client
	log       *logrus.Entry
}

type httpRangeResponse struct {
	unit  string
	start int64
	end   int64 // including
	size  int64
}

type httpRangeRequest struct {
	unit  string
	start int64
	end   int64 // including
}

var errRangeUnsupported = errors.New("http range unsupported")

// ReadAt implements backend.Backend.
func (h *httpBackend) ReadAt(p []byte, off int64) (n int, err error) {
	defer func() {
		h.log.Debugf("read-at 0x%x size=0x%x => (0x%x, %v)", off, len(p), n, err)
		if err != nil {
			h.log.Errorf("read-at 0x%x size=0x%x: %v", off, len(p), err)
		}
	}()
	req, err := http.NewRequest(http.MethodGet, h.url, nil)
	if err != nil {
		return -1, err
	}

	rngReq, err := buildHttpRange(off, int64(len(p)), h.rangeUnit)
	if err != nil {
		return -1, err
	}

	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Range_requests
	req.Header.Add("Range", rngReq.toString())
	resp, err := h.client.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	rngResp, err := parseHttpRange(resp.Header.Get("Content-Range"))
	if err != nil {
		return -1, err
	}

	if rngResp.getSize() != resp.ContentLength {
		return -1, errors.New("corrupt response")
	}
	return io.ReadAtLeast(resp.Body, p, int(resp.ContentLength))
}

// Size implements backend.Backend.
func (h *httpBackend) Size() (int64, error) {
	resp, err := h.client.Head(h.url)
	if err != nil {
		return -1, err
	}

	_size := resp.Header.Get("Content-Length")
	size, err := strconv.ParseInt(_size, 10, 64)
	h.log.Debugf("size %v", size)
	return size, err
}

// Sync implements backend.Backend.
func (h *httpBackend) Sync() error {
	return nil
}

// WriteAt implements backend.Backend.
func (h *httpBackend) WriteAt(p []byte, off int64) (n int, err error) {
	return 0, nil
}

var _ backend.Backend = (*httpBackend)(nil)

func buildHttpRange(off int64, size int64, unit string) (*httpRangeRequest, error) {
	// TODO: other unit support
	assert(unit == "bytes", "unit unimplemented")

	// detect offset+size overflow
	// check zero-read
	locEnd := off + size - 1
	if locEnd < off || off < 0 {
		return nil, errors.New("invalid offset")
	}

	return &httpRangeRequest{
		unit:  unit,
		start: off,
		end:   locEnd,
	}, nil
}

func (r httpRangeResponse) getSize() int64 {
	return r.end - r.start + 1
}

func (rr httpRangeRequest) toString() string {
	return fmt.Sprintf("%s=%v-%v", rr.unit, rr.start, rr.end)
}

func parseHttpRange(rng string) (r httpRangeResponse, err error) {
	// TODO: other unit support
	pat := regexp.MustCompile(`^bytes (\d+)-(\d+)/(\d+)$`)
	arr := pat.FindAllStringSubmatch(rng, -1)
	if len(arr) != 1 {
		return r, errRangeUnsupported
	}
	r.unit = "bytes"
	r.start, _ = strconv.ParseInt(arr[0][1], 10, 64)
	r.end, _ = strconv.ParseInt(arr[0][2], 10, 64)
	r.size, _ = strconv.ParseInt(arr[0][3], 10, 64)
	return r, nil
}
