package iox

import (
	"bytes"
	"errors"
	"hash"
	"io"
	"sync"
	"time"
)

type StatReader struct {
	r             io.Reader
	last, current int64
	total         int64
	deadline      time.Time
	interval      time.Duration
	mu            sync.Mutex
}

func NewStatReader(r io.Reader, interval time.Duration) *StatReader {
	return &StatReader{
		r:        r,
		interval: interval,
	}
}

func (r *StatReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if now.After(r.deadline) {
		r.deadline = now.Add(r.interval)
		r.last = r.current
		r.current = 0
	}
	r.current += int64(n)
	r.total += int64(n)
	return
}

func (r *StatReader) Rate() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	interval := r.interval.Seconds()
	currentRate := float64(r.current) / interval
	currentWeight := float64(r.interval-time.Until(r.deadline)) / float64(r.interval)
	lastRate := float64(r.last) / interval
	lastWeight := 1 - currentWeight
	return lastRate*lastWeight + currentRate*currentWeight
}

func (r *StatReader) Total() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.total
}

type checksumReader struct {
	r   io.Reader
	h   hash.Hash
	sum []byte
}

func NewChecksumReader(r io.Reader, h hash.Hash, sum []byte) io.Reader {
	return &checksumReader{
		r:   io.TeeReader(r, h),
		h:   h,
		sum: sum,
	}
}

func (r *checksumReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	if err == io.EOF && bytes.Equal(r.h.Sum(nil), r.sum) == false {
		err = errors.New("validation failed")
		return
	}
	return
}
