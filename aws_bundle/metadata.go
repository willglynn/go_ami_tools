package aws_bundle

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
)

type Metadata struct {
	Name         string          // restrictions unclear; probably best to stick to [A-Za-z0-9-_.]+
	Architecture string          // "x86_64" or "i386"
	AWSAccountID string          // just digits, no dashes
	AWSRegion    string          // the region to which this bundle this will be sent for registration
	UserKey      *rsa.PrivateKey // an optional private key, in case you'd like to decrypt the bundle later
	Type         string          // assumed to be "machine" if unspecified

	Bundler Application
}

type Application struct {
	Name    string `xml:"name"`    // e.g. "ec2-ami-tools"
	Version string `xml:"version"` // e.g. "1.5"
	Release string `xml:"release"` // e.g. "7"

	Comment string `xml:",comment"` // optional XML comment
}

func (md Metadata) toManifest() manifest {
	m := manifest{
		Bundler: md.Bundler,
		MachineConfiguration: manifestMachineConfig{
			Architecture: md.Architecture,
		},
		Image: manifestImage{
			Name: md.Name,
			User: md.AWSAccountID,
			Type: md.Type,
		},
	}

	// Add defaults
	if m.Image.Type == "" {
		m.Image.Type = "machine"
	}

	return m
}

func (md Metadata) WriteManifest(bundle *Writer, sink Sink) error {
	// Generate a manifest struct
	m := md.toManifest()

	// Ask the Writer to populate the appropriate bits
	bundle.populateManifest(&m)

	// Generate a user key if the caller didn't provide one
	// (This doesn't do any good if the user wants to decrypt their image
	// later, but does anyone actually do that?)
	userKey := md.UserKey
	if userKey == nil {
		if key, err := rsa.GenerateKey(rand.Reader, 1024); err != nil {
			return err
		} else {
			userKey = key
		}
	}

	// Ask the manifest to encrypt the bundle's key and IV for both the target region and the user
	if err := m.EncryptSecrets(bundle.key, bundle.iv, md.AWSRegion, &userKey.PublicKey); err != nil {
		return err
	}

	// Finalize the manifest
	manifestBytes, err := m.SignAndMarshal(userKey)
	if err != nil {
		return err
	}

	// Write the manifest
	if writer, err := sink.WriteBundleFile(fmt.Sprintf("%s.manifest.xml", bundle.basename)); err != nil {
		return err
	} else if n, err := writer.Write(manifestBytes); err != nil {
		writer.Close()
		return err
	} else if n < len(manifestBytes) {
		writer.Close()
		return fmt.Errorf("short manifest write: %d vs %d", n, len(manifestBytes))
	} else if err := writer.Close(); err != nil {
		return err
	}

	// Success!
	return nil
}
