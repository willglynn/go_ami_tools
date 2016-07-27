package aws_bundle

import (
	"fmt"
	"io"
)

// chunkWriter is an io.Writer which delegates to a Sink.
//
// Incoming bytes are automatically split across files exactly chunkSize
// bytes in length.
type chunkWriter struct {
	sink      Sink
	name      string
	chunkSize int

	current struct {
		filename string
		w        io.WriteCloser

		index  int // which chunk is this?
		offset int // how far in are we?
	}

	sha1 map[string]string
}

func newChunkWriter(sink Sink, name string, chunkSize int) *chunkWriter {
	return &chunkWriter{
		sink:      sink,
		name:      name,
		chunkSize: chunkSize,

		sha1: make(map[string]string),
	}
}

func (cw *chunkWriter) Write(p []byte) (n int, err error) {
	for len(p) > 0 {
		// we have something to write
		// how many bytes can we write in this chunk?
		bytes := cw.bytesRemainingInChunk()
		if bytes == 0 {
			// rotate
			cw.newChunk()
		} else {
			// determine how many bytes we want to write
			if bytes > len(p) {
				bytes = len(p)
			}

			// split the buffer
			now, later := p[:bytes], p[bytes:]
			p = later

			// write it
			thisN, thisErr := cw.current.w.Write(now)
			n += thisN
			cw.current.offset += thisN

			// handle errors
			if thisErr != nil {
				return n, thisErr
			}
		}
	}

	return n, nil
}

func (cw *chunkWriter) Close() error {
	if cw.current.w != nil {
		return cw.closeChunk()
	}

	return nil
}

func (cw *chunkWriter) closeChunk() error {
	err := cw.current.w.Close()
	cw.current.w = nil
	return err
}

func (cw *chunkWriter) newChunk() error {
	if cw.current.w != nil {
		if err := cw.closeChunk(); err != nil {
			return err
		}
	}

	cw.current.filename = fmt.Sprintf("%s.part.%d", cw.name, cw.current.index)
	cw.current.index++
	cw.current.offset = 0
	if w, err := cw.sink.WriteBundleFile(cw.current.filename); err != nil {
		return err
	} else {
		cw.current.w = w
	}

	return nil
}

func (cw *chunkWriter) bytesRemainingInChunk() int {
	if cw.current.w == nil {
		// no current chunk
		return 0
	} else {
		return cw.chunkSize - cw.current.offset
	}
}
