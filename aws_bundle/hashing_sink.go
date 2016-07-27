package aws_bundle

import (
	"crypto/sha1"
	"hash"
	"io"
	"sync"
)

type hashingSink struct {
	sync.Mutex
	sink  Sink
	files []hashingSinkFile
}

type hashingSinkFile struct {
	filename string
	hash     []byte
}

func newHashingSink(sink Sink) *hashingSink {
	return &hashingSink{
		sink: sink,
	}
}

type hashingSinkWriter struct {
	sink *hashingSink
	name string
	h    hash.Hash
	w    io.WriteCloser
}

func (h *hashingSink) WriteBundleFile(filename string) (io.WriteCloser, error) {
	// delegate
	w, err := h.sink.WriteBundleFile(filename)
	if err != nil {
		return w, err
	}

	// wrap the WriteCloser with one that calculates hashes on close
	hsw := hashingSinkWriter{
		sink: h,
		name: filename,
		h:    sha1.New(),
		w:    w,
	}
	return &hsw, nil
}

func (hsw *hashingSinkWriter) Write(p []byte) (n int, err error) {
	// write to the hash
	if n, err = hsw.h.Write(p); err != nil {
		return n, err
	}

	// delegate
	return hsw.w.Write(p)
}

func (hsw *hashingSinkWriter) Close() error {
	// finish the hash
	file := hashingSinkFile{
		filename: hsw.name,
		hash:     hsw.h.Sum(nil),
	}

	// record this file on the hashing sink
	hsw.sink.Lock()
	hsw.sink.files = append(hsw.sink.files, file)
	hsw.sink.Unlock()

	// delegate
	return hsw.w.Close()
}
