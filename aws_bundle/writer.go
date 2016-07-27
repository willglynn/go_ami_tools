package aws_bundle

import (
	"archive/tar"
	"crypto/rand"
	"crypto/sha1"
	"errors"
	"fmt"
	"hash"
	"io"
	"time"

	gzip "github.com/klauspost/pgzip"
)

// aws_bundle.Writer writes an input stream as a bundle suitable for use in
// EC2.
//
// Bundle ceration involves a series of transformations -- encapsulation in a
// tar archive, gzip compression, encryption using AES-128-CBC, and splitting
// into as many fixed-size output streams as required.
//
// The bundle format requires the image size to be specified up-front, but in
// constrast to Amazon's `ec2-bundle-image`, there is no requirement to create
// temporary files, to read the image more than once, or to re-read the bundled
// output.
//
// aws_bundle.Writer is therefore an io.WriteCloser, which ultimately causes
// writes to io.WriteClosers provided by a Sink. Writer performs some
// internal buffering, so be sure to handle any errors returned by Close().
//
// Once an aws_bundle.Writer has been closed successfully, the bundle files are
// fully written, and the Writer will no longer interact with its sink.
// You will also require a manifest for the bundle to be useful; see
// Metadata.WriteManifest() for details.
type Writer struct {
	basename string
	size     int64
	sink     Sink

	sha1 hash.Hash
	hs   *hashingSink
	cw   io.WriteCloser
	aes  io.WriteCloser
	gz   io.WriteCloser
	tar  *tar.Writer

	bundledSize *countingWriter
	trueSize    *countingWriter

	didInitialWrite bool
	closed          bool

	key []byte
	iv  []byte
}

// NewWriter() returns an aws_bundle.Writer.
//
// The resulting files will be named according to basename, e.g.
// "basename.part.0". These files will be written to the Sink you provide.
//
// The AWS bundle format requires the size to be specified before any data is
// written, so you must supply it here.
func NewWriter(basename string, size int64, sink Sink) (*Writer, error) {
	// Bundling an AMI requires a processing chain on the image stream:
	// 1. tar the image
	// 2. gzip the tarred image
	// 3. encrypt the gzipped tarred image
	// 4. split the encrypted gzipped tarred image into 10 MiB chunks
	//
	// Additionally, we must
	// - SHA1 the tarred image in its entirety,
	// - SHA1 each encrypted gzipped tarred chunk,
	// - count the total number of bytes in, and
	// - count the total number of bytes out
	// in order to generate a manifest.

	// Start by making a Writer struct, since we'll need that
	bw := Writer{
		basename: basename,
		size:     size,
		sink:     sink,

		key: make([]byte, 16),
		iv:  make([]byte, 16),
	}

	// Generate some random secrets
	if _, err := rand.Read(bw.key); err != nil {
		return nil, err
	}
	if _, err := rand.Read(bw.iv); err != nil {
		return nil, err
	}

	// Now, build the processing chain bottom-up:
	// - a hashingSink calculates SHA1s for each chunk and writes to the output sink
	// - a chunkWriter breaks the stream into parts and writes to the hashingSink
	// - a "bundledSize" countingWriter counts the number of bytes out
	// - an aesCbcWriter encrypts the stream and writes to the chunkWriter
	// - a gzip.Writer compresses the stream and writes to the aesCbcWriter
	// - an io.MultiWriter which writes to both a SHA1 hash and the gzip.Writer
	// - a tar.Writer emits a tar header and then writes to the tee
	// - a "trueSize" countingWriter counts the number of bytes in for later comparison
	bw.hs = newHashingSink(sink)
	bw.cw = newChunkWriter(bw.hs, bw.basename, 10*1024*1024)
	bw.bundledSize = newCountingWriter(bw.cw)
	if aes, err := newAes128CbcWriter(bw.bundledSize, bw.key, bw.iv); err != nil {
		return nil, err
	} else {
		bw.aes = aes
	}
	if gz, err := gzip.NewWriterLevel(bw.aes, gzip.BestCompression); err != nil {
		return nil, err
	} else {
		gz.SetConcurrency(256<<10, 32) // up to 32x 256 KB buffers in flight
		bw.gz = gz
	}
	bw.sha1 = sha1.New()
	tee := io.MultiWriter(bw.sha1, bw.gz)
	bw.tar = tar.NewWriter(tee)
	bw.trueSize = newCountingWriter(bw.tar)

	return &bw, nil
}

func (bw *Writer) doInitialWrite() error {
	hdr := tar.Header{
		Name:     bw.basename,
		Mode:     0644,
		Uid:      0,
		Gid:      0,
		Uname:    "root",
		Gname:    "root",
		Size:     bw.size,
		ModTime:  time.Now(),
		Typeflag: 0x30,
	}

	err := bw.tar.WriteHeader(&hdr)
	if err == nil {
		bw.didInitialWrite = true
	}
	return err
}

// Write bytes to the bundle.
func (bw *Writer) Write(p []byte) (n int, err error) {
	if !bw.didInitialWrite {
		if err := bw.doInitialWrite(); err != nil {
			return 0, err
		}
	}

	// Forward bytes into the top of the chain
	return bw.trueSize.Write(p)
}

// Close the bundle. Closing more than once is an error.
//
// Close() flushes all internal buffers, adds endings to various data
// structures, finalizes several hashes, etc. In other words, Close() causes
// writes. Check the return value.
func (bw *Writer) Close() error {
	if bw.closed {
		return errors.New("Writer is already closed")
	}

	errors := []error{}

	// close the tar file, which does not close the underlying writer
	if err := bw.tar.Close(); err != nil {
		errors = append(errors, err)
	}

	// close the gzip stream, which does not close the underlying writer
	if err := bw.gz.Close(); err != nil {
		errors = append(errors, err)
	}

	// close the AES stream
	if err := bw.aes.Close(); err != nil {
		errors = append(errors, err)
	}

	// close the chunkWriter, which *does* bubble down through the remaining layers
	if err := bw.cw.Close(); err != nil {
		errors = append(errors, err)
	}

	// check that the image we wrote was exactly the size we promised in the tar header
	if bw.size != bw.trueSize.n {
		errors = append(errors, fmt.Errorf("expected %d bytes, actually wrote %d bytes", bw.size, bw.trueSize.n))
	}

	bw.closed = true

	if len(errors) > 0 {
		// return the first error, since that is likely the underlying cause
		return errors[0]
	} else {
		// no errors
		return nil
	}
}

func (bw *Writer) populateManifest(m *manifest) {
	// Fill in the scalars
	m.Image.Digest.Algorithm = "SHA1"
	m.Image.Digest.Value = fmt.Sprintf("%x", bw.sha1.Sum(nil))

	m.Image.Size = bw.trueSize.n
	m.Image.BundledSize = bw.bundledSize.n

	// Populate parts from the hashing sink
	for i, file := range bw.hs.files {
		part := manifestPart{
			Index:    i,
			Filename: file.filename,
			Digest: valueAndAlgorithm{
				Value:     fmt.Sprintf("%x", file.hash),
				Algorithm: "SHA1",
			},
		}
		m.Image.PartsContainer.Parts = append(m.Image.PartsContainer.Parts, part)
	}
	m.Image.PartsContainer.Count = len(bw.hs.files)
}
