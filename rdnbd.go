package rdnbd

import (
	"context"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/pojntfx/go-nbd/pkg/backend"
	"github.com/pojntfx/go-nbd/pkg/client"
	"github.com/pojntfx/go-nbd/pkg/server"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	cfg Config
	b   backend.Backend
}

type Config struct {
	URL             string
	Device          string
	Cache           string
	CacheSize       int64
	LogCacheMetrics bool
}

func New(cfg Config) *Server {
	return &Server{
		cfg: cfg,
		b:   nil,
	}
}

func (s *Server) Run() (err error) {
	logrus.Infof("rdnbd serve %s", s.cfg.URL)

	file, err := os.CreateTemp("", "rdnbd-")
	if err != nil {
		return err
	}
	unixSockPath := file.Name()
	file.Close()
	os.Remove(unixSockPath)

	lis, err := net.Listen("unix", unixSockPath)
	if err != nil {
		return err
	}
	defer lis.Close()
	logrus.Infof("listen on unix:%s", unixSockPath)

	s.b = &httpBackend{
		url:       s.cfg.URL,
		rangeUnit: "bytes",
		client:    &http.Client{},
		log:       logrus.WithField("module", "http-backend"),
	}
	if s.cfg.Cache != "" {
		cache := &cacheBackend{
			b:         s.b,
			cache:     s.cfg.Cache,
			blockSize: 512,
			log:       logrus.WithField("module", "cache"),
		}
		if err := cache.init(); err != nil {
			logrus.Error(err)
			return err
		}
		s.b = cache
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if s.cfg.LogCacheMetrics {
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-time.After(5 * time.Second):
						cache.metrics.ShowMetrics(cache.log)
					}
				}
			}()
		}
	}

	eg := errgroup.Group{}
	eg.Go(func() error {
		return s.serveNBD(lis)
	})
	eg.Go(func() error {
		return s.connect(unixSockPath)
	})
	return eg.Wait()
}

func (s *Server) serveNBD(lis net.Listener) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()
	for {
		conn, err := lis.Accept()
		must(err)

		logrus.Infof("accept connection from %s", conn.RemoteAddr())
		err = server.Handle(conn, []*server.Export{
			{
				Name:        "default",
				Description: "rdnbd",
				Backend:     s.b,
			},
		}, &server.Options{
			ReadOnly:           true,
			MinimumBlockSize:   512,
			PreferredBlockSize: 2 << 20,
			MaximumBlockSize:   64 << 20, // 64M
			MaximumRequestSize: 512 << 20,
			SupportsMultiConn:  false,
		})
		must(err)
		conn.Close()
	}
}

func (s *Server) connect(addr string) (err error) {
	defer func() {
		logrus.Infof("connect done: %v", err)
	}()
	conn, err := net.Dial("unix", addr)
	if err != nil {
		return err
	}

	dev, err := os.OpenFile(s.cfg.Device, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	logrus.Infof("connect %s as %s", addr, s.cfg.Device)
	return client.Connect(conn, dev, &client.Options{
		ExportName: "default",
		BlockSize:  512,
	})
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func assert(cond bool, msg any) {
	if !cond {
		panic(msg)
	}
}
