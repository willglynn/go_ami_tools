package main

import (
	"compress/bzip2"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/willglynn/go_ami_tools/aws_bundle"
	"github.com/willglynn/go_ami_tools/aws_bundle_glue"
)

var config struct {
	// source
	image string

	// metadata
	name         string
	architecture string
	account      string
	region       string

	// sink
	bucket string
	prefix string
}

func init() {
	flag.StringVar(&config.image, "image", "", "filename of disk image to bundle/upload")
	flag.StringVar(&config.name, "name", "image", "basename to use in resulting image")
	flag.StringVar(&config.architecture, "arch", "x86_64", "CPU architecture (\"x86_64\" or \"i386\")")
	flag.StringVar(&config.account, "account", "", "AWS account number (without dashes)")
	flag.StringVar(&config.bucket, "s3-bucket", "", "S3 bucket to which the image should be uploaded")
	flag.StringVar(&config.prefix, "s3-prefix", "", "prefix to use within the S3 bucket (optional, should probably end with \"/\" if specified)")
	flag.StringVar(&config.region, "region", "", "region to use for S3 upload and image manifest (determined automatically from S3 bucket)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  %s -image <path/to/disk/image> -s3-bucket <bucket name>\n\nFull parameters:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
-image must reference a bootable disk image file. If the filename ends in
.gz or .bz2, it will be transparently decompressed.

ec2-bundle-and-upload-image searches for credentials the usual way. Specify
AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY environment variables, put keys in
~/.aws/credentials (optionally scoped by AWS_PROFILE), or run it on an EC2
instance with an appropriate IAM profile.

Usage requires the following AWS permissions:

	s3:PutObject           to upload the bundle
	s3:GetBucketLocaion    (if -region is unspecified)
	sts:GetCallerIdentity  (if -account is unspecified)

`)
	}
}

// requires s3:GetBucketLocation
func determineRegion() {
	if config.bucket == "" {
		return
	}

	// talk to S3 in  us-east-1
	s3Svc := s3.New(session.New(), aws.NewConfig().WithRegion("us-east-1"))

	// ask it where the target bucket is
	input := s3.GetBucketLocationInput{
		Bucket: &config.bucket,
	}
	output, err := s3Svc.GetBucketLocation(&input)

	// blow up if it failed
	if err != nil {
		log.Fatal("Unable to s3:GetBucketLocation; please specify -region", err)
	}

	if output.LocationConstraint != nil {
		config.region = *output.LocationConstraint
	} else {
		// looks like us-east-1
		config.region = "us-east-1"
	}

	log.Printf("Using \"-region %s\" to match S3 bucket", config.region)
}

// requires sts:GetCallerIdentity
func determineAccount() {
	stsSvc := sts.New(session.New(), aws.NewConfig().WithRegion("us-east-1"))

	input := sts.GetCallerIdentityInput{}
	output, err := stsSvc.GetCallerIdentity(&input)

	// blow up if it failed
	if err != nil || output == nil || output.Account == nil {
		log.Fatal("Unable to sts:GetCallerIdentity; please specify -account", err)
	} else {
		config.account = *output.Account
	}

	log.Printf("Using \"-account %s\" based on active credentials", config.account)
}

type loggingSink struct {
	sink aws_bundle.Sink
}

func (ls loggingSink) WriteBundleFile(filename string) (io.WriteCloser, error) {
	log.Printf("Writing to s3://%s/%s%s", config.bucket, config.prefix, filename)
	return ls.sink.WriteBundleFile(filename)
}

func sizeByReadingUntilEOF(r io.Reader) (int64, error) {
	log.Print("Determining size of compressed image...")

	var bytes int64

	// make a 256 KB buffer
	buf := make([]byte, 256<<10)

	for {
		// read
		n, err := r.Read(buf)
		bytes += int64(n)

		if err == io.EOF {
			// we succeeded
			log.Printf("Compressed image is %d bytes", bytes)
			return bytes, nil
		} else if err != nil {
			// we failed
			return bytes, err
		}
	}
}

type compressedFile struct {
	decompressor io.Reader
	file         *os.File
}

func (cf *compressedFile) Read(p []byte) (n int, err error) {
	return cf.decompressor.Read(p)
}

func (cf *compressedFile) Close() error {
	return cf.file.Close()
}

// open the file, potentially decompressing it
func open(filename string) (io.ReadCloser, int64, error) {
	// open
	f, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}

	if strings.HasSuffix(filename, ".bz2") {
		// determine size
		size, err := sizeByReadingUntilEOF(bzip2.NewReader(f))
		if err != nil {
			f.Close()
			return nil, 0, err
		}

		// rewind
		if _, err := f.Seek(0, os.SEEK_SET); err != nil {
			f.Close()
			return nil, 0, err
		}

		// return
		return &compressedFile{
			decompressor: bzip2.NewReader(f),
			file:         f,
		}, size, nil

	} else if strings.HasSuffix(filename, ".gz") {
		// determine size
		r, err := gzip.NewReader(f)
		if err != nil {
			return nil, 0, err
		}
		size, err := sizeByReadingUntilEOF(r)
		if err != nil {
			f.Close()
			return nil, 0, err
		}

		// rewind
		if _, err := f.Seek(0, os.SEEK_SET); err != nil {
			f.Close()
			return nil, 0, err
		}

		// open again
		r, err = gzip.NewReader(f)
		if err != nil {
			return nil, 0, err
		}

		// return
		return &compressedFile{
			decompressor: r,
			file:         f,
		}, size, nil

	} else {
		// stat to determine size
		fi, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, 0, err
		}
		size := fi.Size()

		// return
		return f, size, nil
	}
}

func main() {
	flag.Parse()

	// validate parameters
	if config.image == "" || config.bucket == "" {
		fmt.Fprintf(os.Stderr, "Error: both -image and -s3-bucket must be specified\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// guess config as needed
	if config.region == "" {
		determineRegion()
	}
	if config.account == "" {
		determineAccount()
	}
	if config.name == "" {
		config.name = "image"
	}

	// open the image
	image, size, err := open(config.image)
	if err != nil {
		log.Fatalf("Unable to open image: %v", err)
	}

	// set up the sink
	s3Svc := s3.New(session.New(), aws.NewConfig().WithRegion(config.region))
	sink := &loggingSink{
		sink: aws_bundle_glue.NewS3Sink(s3Svc, config.bucket, config.prefix),
	}

	// set up the bundle writer
	writer, err := aws_bundle.NewWriter(config.name, size, sink)
	if err != nil {
		log.Fatalf("Error starting bundle write: %v", err)
	}

	// copy the image to the bundle writer
	if n, err := io.Copy(writer, image); err != nil {
		log.Fatalf("Error after %d bytes: %v", n, err)
	}

	// close the bundle writer
	if err := writer.Close(); err != nil {
		log.Fatalf("Error closing bundle: %v", err)
	}

	// build the metadata
	meta := aws_bundle.Metadata{
		Name:         config.name,
		Architecture: config.architecture,
		AWSAccountID: config.account,
		AWSRegion:    config.region,

		Bundler: aws_bundle.Application{
			Name:    "ec2-bundle-and-upload-image",
			Version: "0.1",
			Release: "1",
		},
	}

	// turn it into a manifest
	if err := meta.WriteManifest(writer, sink); err != nil {
		log.Fatalf("Error writing manifest: %v", err)
	}

	// done!
	manifestLocation := fmt.Sprintf("%s/%s%s.manifest.xml", config.bucket, config.prefix, config.name)
	log.Printf("Bundle creation/upload complete.")
	log.Printf("Register your new AMI using e.g.:")
	log.Printf("  `aws ec2 register-image --name %q --virtualization-type=hvm --block-device-mappings \"VirtualName=ami,DeviceName=sda VirtualName=ephemeral0,DeviceName=sdb\" --root-device=/dev/xvda --image-location %s`", path.Base(config.image), manifestLocation)
	log.Printf("Printing image location to standard output and terminating\n")
	fmt.Printf("%s\n", manifestLocation)
}
