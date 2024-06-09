package rdnbd

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync/atomic"
	"syscall"

	"github.com/pojntfx/go-nbd/pkg/backend"
	"github.com/sirupsen/logrus"
)

type cacheBackend struct {
	b         backend.Backend
	cache     string
	blockSize int
	log       *logrus.Entry

	cacheFp *os.File
	indexFp *os.File
	indexM  []byte
	metrics cacheMetrics
}

type cacheMetrics struct {
	HitCount   atomic.Int64
	TotalCount atomic.Int64
}

func (c *cacheBackend) init() error {
	var err error
	logrus.Info("cache init")

	c.cacheFp, err = os.OpenFile(c.cache, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	sz, err := c.b.Size()
	if err != nil {
		return err
	}
	block := sz / int64(c.blockSize)
	indexFile := c.cache + ".idx"
	c.indexFp, err = os.OpenFile(indexFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	assert((block*4) <= (2<<31)-1, "block count too large")
	c.indexFp.Truncate(block * 4)
	c.indexM, err = syscall.Mmap(
		int(c.indexFp.Fd()),
		0, int(block*4),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED)
	return err
}

func (c *cacheBackend) sync() {
	addr := reflect.ValueOf(c.indexM).UnsafePointer()
	syscall.Syscall(syscall.SYS_MSYNC, uintptr(addr), uintptr(len(c.indexM)), syscall.MS_SYNC)
	c.indexFp.Sync()
	c.cacheFp.Sync()
}

// ReadAt implements backend.Backend.
func (c *cacheBackend) ReadAt(p []byte, off int64) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
		c.log.Debugf("read-at 0x%x size=0x%x => 0x%x %v", off, len(p), n, err)
	}()
	assert(len(p)%c.blockSize == 0, fmt.Errorf("invalid read size"))
	assert(off%int64(c.blockSize) == 0, fmt.Errorf("invalid offset"))

	block := off / int64(c.blockSize)
	count := len(p) / c.blockSize
	c.ensureCache(uint32(block), count, p)
	return len(p), nil
}

func (c *cacheBackend) ensureCache(block uint32, count int, buffer []byte) {
	// split block request
	var uncached []blockReqest
	c.metrics.TotalCount.Add(int64(count))
	for i := 0; i < count; i++ {
		// MAYBE BUG
		blockID := block + uint32(i)
		idx := binary.BigEndian.Uint32(c.indexM[blockID*4:])
		// check cahce
		if idx != 0 {
			c.metrics.HitCount.Add(1)
			off := int64(idx-1) * int64(c.blockSize)
			n, err := c.cacheFp.ReadAt(buffer[i*c.blockSize:(i+1)*c.blockSize], off)
			must(err)
			assert(n == c.blockSize, fmt.Errorf("unexpected eof: %v < %v", n, c.blockSize))
			continue
		}
		if len(uncached) > 0 {
			// merge continue request
			last := &uncached[len(uncached)-1]
			if last.id+uint32(last.count) == blockID {
				last.count++
				continue
			}
		}
		uncached = append(uncached, blockReqest{
			id:     blockID,
			count:  1,
			buffer: buffer[i*c.blockSize:],
		})
	}

	// ensure cache
	for _, req := range uncached {
		c.readUncachedBlock(req)
	}
	if len(uncached) > 0 {
		c.sync()
	}
}

type blockReqest struct {
	id     uint32
	count  int
	buffer []byte
}

func (c *cacheBackend) readUncachedBlock(req blockReqest) {
	off := int64(req.id) * int64(c.blockSize)
	sz := req.count * c.blockSize
	n, err := c.b.ReadAt(req.buffer[:sz], off)
	must(err)
	assert(n == sz, fmt.Errorf("unexpected eof: %v < %v", n, sz))

	cacheOff, err := c.cacheFp.Seek(0, io.SeekEnd)
	must(err)
	_, err = c.cacheFp.Write(req.buffer[:sz])
	must(err)

	idx := uint32(cacheOff/int64(c.blockSize)) + 1
	for i := uint32(0); i < uint32(req.count); i++ {
		binary.BigEndian.PutUint32(c.indexM[(req.id+i)*4:], idx+i)
	}
}

// Size implements backend.Backend.
func (c *cacheBackend) Size() (int64, error) {
	return c.b.Size()
}

// Sync implements backend.Backend.
func (c *cacheBackend) Sync() error {
	return nil
}

// WriteAt implements backend.Backend.
func (c *cacheBackend) WriteAt(p []byte, off int64) (n int, err error) {
	return 0, nil
}

var _ backend.Backend = (*cacheBackend)(nil)

func (m *cacheMetrics) ShowMetrics(log *logrus.Entry) {
	var (
		hit     = m.HitCount.Load()
		total   = m.TotalCount.Load()
		hitRate = float64(hit) / float64(total)
	)
	log.Infof("cache metrics: hit=%v total=%v hit-rate: %.2f",
		hit, total, hitRate)
}
