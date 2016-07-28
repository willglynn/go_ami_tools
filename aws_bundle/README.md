`aws_bundle` package
====================

Using `ec2-ami-tools` is a pain. This package may ease your pain.

There are two conventional ways to produce an instance store AMI:
`ec2-bundle-vol` which asks an EC2 instance to bundle itself for use as
an AMI, and `ec2-bundle-image` which takes a disk image and bundles it for
use as an AMI. In either case, you end up with a bunch of files on disk which
you then pass to `ec2-upload-bundle`, which gives you a manifest URL which you
then pass to `ec2-api-tools`' `ec2-register` or `aws-cli`'s
`aws ec2 register-image`.

This `aws_bundle` package is a pure Go alternative to `ec2-bundle-image`: give
it a disk image, it'll produce a bundle. You can send the bundle (and the
corresponding manifest) to S3 and register the image using the
[normal AWS SDK](https://github.com/aws/aws-sdk-go). There's no magic.

Creating an instance store image does _not_ require an X.509 signing
certificate. (That's `ec2-ami-tools` talking.) All you need is regular AWS
credentials with `s3:PutObject` and `ec2:RegisterImage` access.

The Process
-----------

Generating an instance store AMI requires:

* Making a disk image. `ec2-bundle-vol` does this by `mount -o loop`, copying
  everything, and then `umount`ing. `ec2-bundle-image` assumes you did this
  yourself.
* Turning that disk image into a bundle. Both `ec2-bundle-*` tools do this.
* Uploading the bundle to S3 with `x-amz-acl: aws-exec-read`.
* Registering the bundle manifest with EC2.

There are many tools to make disk images, and many SDKs to upload files to S3
and register manifests with EC2. There are not many ways to turn disk images
to bundles – especially none that are pure Go – which is why this code exists.

Usage
-----

The `aws_bundle` package attemps to accomodate as many use cases as possible.
It therefore has no dependencies on the AWS SDK, it makes no use of the
network, and there's no reason at all it would need to run inside EC2.

Implement the `aws_bundle.Sink` interface to handle the bundle output. You can
write the results to disk, or stream them straight to S3, or whatever you like,
just express it as a `Sink`:

```
type Sink interface {
	WriteBundleFile(filename string) (io.WriteCloser, error)
}
```

To make a bundle, get an `aws_bundle.Writer`, `Write()` the raw disk image to
it, `Close()`. Easy.

`aws_bundle.NewWriter()` takes:
   
   * a `basename`, which determines the names of the files it produces
   * a `size` in bytes, which is needed up front because of AWS design decisions
   * a `sink` to which the `Writer` should write

In order to use the bundle, you'll also need a manifest file. Manifests contain
various metadata, like the machine's architecture, the image's owner, etc.
Fill out an `aws_bundle.Metadata` structure as appropriate and call
`WriteManifest()`, providing both the closed `aws_bundle.Writer` and a `sink`.

Cryptography
------------

`ec2-ami-tools` requires you to specify an X.509 certificate and private key,
and the documentation variously states or implies that this must be an
Amazon-registered signing certificate. This is not actually a requirement.

Bundles are encrypted with AES-128-CBC using a random key and initialization
vector. These secrets are then encrypted using EC2's RSA public key and
included in the manifest file. When you launch an instance, EC2 retrieves the
manifest, decrypts these secrets using its RSA private key, and uses the AES
key + IV to decrypt the bundle itself.

In addition to the EC2-facing AES key and IV, the manifest also contains a
second copy of the key and IV, this time encrypted with the user's RSA public
key. This permits users to download a bundle and decrypt it again at some
point in the future, provided they still have that RSA key.

Finally, the manifest includes a signature, in which the user's RSA private
key signs most of the manifest. Amazon has no way to verify this signature;
it's there purely for the user's benefit, so that the user may download and
verify the manifest's signature using the user's RSA key.

So: if you do not need to decrypt your own bundles, and you do not need to
validate your own manifest signatures, then you do not need to provide an RSA
key.

If you _do_ need to do these things, then you still don't need to use an
Amazon-related X.509 certificate or private key. You can use any RSA key of
your choosing.

Manifests and Regions
---------------------

Note that unlike bundles, bundle _manifests_ are specific to particular AWS
regions – or at least they can be. Amazon uses one RSA key to decrypt
manifests in most regions, but they use a different key in `us-gov-west-1` and
`cn-north-1`.

If you're targeting multiple regions with different keys, you may find it
advantageous to distribute the same bundle to all regions and to generate and
register region-specific manifests, versus the alternatives of bundling
multiple times or using `ec2:CopyImage`. You can generate multiple manifests
for the same bundle by calling `WriteManifest()` repeatedly.
