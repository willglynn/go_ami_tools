`ec2-bundle-and-upload-image`
=============================

This tool bundles and uploads a disk image for use as an EC2 instance store
AMI.

Quick start:

    $ go get -u github.com/willglynn/go_ami_tools/...
    $ ec2-bundle-and-upload-image -image disk-image.raw -s3-bucket mybucket

Things it doesn't do:

* It doesn't create temporary files
* It doesn't need any Ruby or Java runtime
* It doesn't use any RSA keys or X.509 certificates

Things it _does_ do:

* It can determine your AWS account ID automatically
* It can determine your target region automatically
* If the `-image` filename ends in `.bz2` or `.gz`, it will decompress
  automatically while bundling

It prints progress and errors to stderr. On success, it'll exit with code 0
and print the bundle manifest location to stdout. You can then register AMI(s)
using that location.

Options
-------

* `-image <filename>`: the image to bundle
* `-s3-bucket <bucket>`: the S3 bucket to use for uploads
* `-s3-prefix <prefix/>`: an optional prefix to use within that bucket
  (you probably want it to end with "/")
* `-region <region>`: the target region
* `-account <123456789012>`: AWS account number, without dashes
* `-arch <x86_64|i386>`: CPU architecture for the bundle (defaults to `x86_64`)

AWS Interface
-------------

`ec2-bundle-and-upload-image` searches for credentials the usual way. Specify
`AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` environment variables, put keys
in `~/.aws/credentials` (optionally scoped by `AWS_PROFILE`), or run it on an
EC2 instance with an appropriate IAM profile.

If `-region` is unspecified, it will be determined based on the S3 bucket's
location (assuming `s3:GetBucketLocation` is permitted).

If `-account` is unspecified, it will be determined based on the current
credentials (assuming `sts:GetCallerIdentity` is permitted).

The image will be processed into a bundle and uploaded directly to S3. This
requires `s3:PutObject` permissions.

`ec2-bundle-and-upload-image` is concerned with getting your image into EC2 in
a way that it can use. Once it's there, you must tell EC2 _how_ to use the
image by registering an AMI. It'll suggest a command for you to use, and it'll
be something like:

    aws ec2 register-image \
    	--name my-fancy-image \
    	--virtualization-type=hvm \
    	--block-device-mappings "VirtualName=ami,DeviceName=sda VirtualName=ephemeral0,DeviceName=sdb" \
    	--root-device=/dev/xvda \
    	--image-location my-bucket/my-prefix/my-fancy-image.manifest.xml

Your disk image might require a different block device mapping string, root
device, virtualization type, or other options. See the
[`aws ec2 register-image` CLI docs](http://docs.aws.amazon.com/cli/latest/reference/ec2/register-image.html)
or the [EC2 RegisterImage API docs](http://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RegisterImage.html)
for reference.
