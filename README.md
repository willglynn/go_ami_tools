`go_ami_tools` [![Build Status](https://travis-ci.org/willglynn/go_ami_tools.svg?branch=master)](https://travis-ci.org/willglynn/go_ami_tools)
==============

This repo contains code that can generate instance store AMIs.

[`ec2-bundle-and-upload-image`](https://github.com/willglynn/go_ami_tools/tree/master/cmd/ec2-bundle-and-upload-image)
is a command-line tool that makes creating instance store AMIs extremely
simple.

(How simple? It can operate with as few as two parameters.)

[`aws_bundle`](https://github.com/willglynn/go_ami_tools/tree/master/aws_bundle)
is a Go package that goes from a disk image to an EC2-compatible bundle.
