package aws_bundle

import "io"

// A Sink is provided by the application to receive data produced by an
// aws_bundle.Writer. Pass back an io.WriteCloser as requested.
type Sink interface {
	WriteBundleFile(filename string) (io.WriteCloser, error)
}
