package aws_bundle_glue

import (
	"io"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type S3Sink struct {
	uploader *s3manager.Uploader
	bucket   string
	prefix   string
}

// NewS3Sink() returns an S3Sink pointing to the specified bucket and prefix.
//
// Prefix is optional, but if specified, it should probably end with a "/".
func NewS3Sink(s3Svc *s3.S3, bucket string, prefix string) *S3Sink {
	uploader := s3manager.NewUploaderWithClient(s3Svc, func(u *s3manager.Uploader) {
		u.PartSize = s3manager.MinUploadPartSize
		u.Concurrency = 8
	})

	return &S3Sink{
		uploader: uploader,
		bucket:   bucket,
		prefix:   prefix,
	}
}

// WriteBundleFile() implements the aws_bundle.Sink interface.
func (sink *S3Sink) WriteBundleFile(filename string) (io.WriteCloser, error) {
	// Make a pipe
	pipeR, pipeW := io.Pipe()

	// Set up an S3 upload reading from half of this pipe
	key := sink.prefix + filename
	contentType := "binary/octet-stream"
	acl := "aws-exec-read"
	input := &s3manager.UploadInput{
		Bucket: &sink.bucket,
		Key:    &key,

		Body:        pipeR,
		ContentType: &contentType,
		ACL:         &acl,
	}

	// Prepare an error channel
	errC := make(chan error, 1)

	// Wrap the write half of this pipe into an s3SinkFile
	f := &s3SinkFile{
		pipe:       pipeW,
		completion: errC,
	}

	// Upload this s3File in the background, returning errors to the file
	go func() {
		_, err := sink.uploader.Upload(input)

		if err != nil {
			errC <- err
		}
		close(errC)
	}()

	return f, nil
}

type s3SinkFile struct {
	pipe       io.WriteCloser
	completion <-chan error
}

func (f *s3SinkFile) Write(p []byte) (n int, err error) {
	return f.pipe.Write(p)
}

func (f *s3SinkFile) Close() error {
	err := f.pipe.Close()
	if err != nil {
		return err
	}

	// wait for the upload to complete, and return any errors
	backgroundErr := <-f.completion
	if backgroundErr != nil {
		return backgroundErr
	}

	return nil
}
