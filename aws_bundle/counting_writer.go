package aws_bundle

import "io"

type countingWriter struct {
	io.Writer
	n int64
}

func newCountingWriter(w io.Writer) *countingWriter {
	return &countingWriter{
		Writer: w,
	}
}

func (cw *countingWriter) Write(p []byte) (n int, err error) {
	n, err = cw.Writer.Write(p)
	cw.n += int64(n)
	return
}
