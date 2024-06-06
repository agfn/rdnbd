package main

import (
	"flag"

	"github.com/agfn/rdnbd"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		url       string
		dev       string
		cache     string
		cacheSize int64
		logLevel  string
	)
	flag.StringVar(&dev, "device", "/dev/nbd0", "nbd device")
	flag.StringVar(&cache, "cache", "", "cache file")
	flag.Int64Var(&cacheSize, "cache-size", 1<<30, "cache size")
	flag.StringVar(&logLevel, "log-level", "info", "log level")
	flag.Parse()
	url = flag.Arg(0)

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("change log level to %s", logLevel)
	logrus.SetLevel(level)

	s := rdnbd.New(rdnbd.Config{
		URL:       url,
		Device:    dev,
		Cache:     cache,
		CacheSize: cacheSize,
	})
	if err := s.Run(); err != nil {
		logrus.Fatal(err)
	}
}
