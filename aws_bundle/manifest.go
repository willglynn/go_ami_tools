package aws_bundle

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/xml"
	"fmt"
)

type manifest struct {
	Bundler Application

	MachineConfiguration manifestMachineConfig

	Image manifestImage
}

type manifestMachineConfig struct {
	XMLName xml.Name `xml:"machine_configuration"`

	Architecture string `xml:"architecture"`

	BlockDeviceMappings []BlockDeviceMapping `xml:"block_device_mapping>mapping"`
}

type manifestImage struct {
	XMLName xml.Name `xml:"image"`

	Name string `xml:"name"`
	User string `xml:"user"`
	Type string `xml:"type"`

	Digest valueAndAlgorithm `xml:"digest"`

	Size        int64 `xml:"size"`
	BundledSize int64 `xml:"bundled_size"`

	EC2EncryptedKey  valueAndAlgorithm `xml:"ec2_encrypted_key"`
	UserEncryptedKey valueAndAlgorithm `xml:"user_encrypted_key"`

	EC2EncryptedIV  string `xml:"ec2_encrypted_iv"`
	UserEncryptedIV string `xml:"user_encrypted_iv"`

	PartsContainer struct {
		Count int            `xml:"count,attr"`
		Parts []manifestPart `xml:"part"`
	} `xml:"parts"`
}

type manifestPart struct {
	Index    int               `xml:"index,attr"`
	Filename string            `xml:"filename"`
	Digest   valueAndAlgorithm `xml:"digest"`
}

type valueAndAlgorithm struct {
	Algorithm string `xml:"algorithm,attr"`
	Value     string `xml:",chardata"`
}

func (m *manifest) EncryptSecrets(key, iv []byte, region string, userKey *rsa.PublicKey) error {
	// We need two public keys: one for EC2, one for the user
	// We were given the user's, so now we just need EC2's
	var ec2key *rsa.PublicKey

	// Look up the EC2 key by region
	if cert, err := CertificateForEC2Region(region); err != nil {
		return fmt.Errorf("unable to get certificate for region %q: %v", region, err)
	} else if key, ok := cert.PublicKey.(*rsa.PublicKey); !ok {
		return fmt.Errorf("certificate for region %q does not contain an RSA key", region)
	} else {
		ec2key = key
	}

	// Okay, we have the RSA keys
	// One other wrinkle: Amazon doesn't want the AES key and IV expressed in binary
	// They want it in lowercase hexadecimal, because reasons.
	encodedKey := []byte(fmt.Sprintf("%x", key))
	encodedIV := []byte(fmt.Sprintf("%x", iv))

	// Encrypt the key for both parties
	if bytes, err := rsa.EncryptPKCS1v15(rand.Reader, ec2key, encodedKey); err != nil {
		return err
	} else {
		m.Image.EC2EncryptedKey = valueAndAlgorithm{
			Value:     fmt.Sprintf("%x", bytes),
			Algorithm: "AES-128-CBC",
		}
	}
	if bytes, err := rsa.EncryptPKCS1v15(rand.Reader, userKey, encodedKey); err != nil {
		return err
	} else {
		m.Image.UserEncryptedKey = valueAndAlgorithm{
			Value:     fmt.Sprintf("%x", bytes),
			Algorithm: "AES-128-CBC",
		}
	}

	// Encrypt the IV for both parties
	if bytes, err := rsa.EncryptPKCS1v15(rand.Reader, ec2key, encodedIV); err != nil {
		return err
	} else {
		m.Image.EC2EncryptedIV = fmt.Sprintf("%x", bytes)
	}
	if bytes, err := rsa.EncryptPKCS1v15(rand.Reader, userKey, encodedIV); err != nil {
		return err
	} else {
		m.Image.UserEncryptedIV = fmt.Sprintf("%x", bytes)
	}

	// Success
	return nil
}

func (m manifest) SignAndMarshal(key *rsa.PrivateKey) ([]byte, error) {
	// The RSA signature is calculated over a SHA1 of the marshalled XML representing
	// <machine_configuration/> concatenated with <image/>.

	// First, encode the manifest in a way that matches what we want to sign
	var signedData bytes.Buffer
	encoder := xml.NewEncoder(&signedData)
	if err := encoder.Encode(m.MachineConfiguration); err != nil {
		return nil, err
	}
	if err := encoder.Encode(m.Image); err != nil {
		return nil, err
	}

	// Generate the signature
	sum := sha1.Sum(signedData.Bytes())
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, sum[:])
	if err != nil {
		return nil, err
	}

	// Pack the signed bytes and the signature into a structure containing the rest of the manifest
	signedManifest := struct {
		XMLName xml.Name `xml:"manifest"`

		Version    string      `xml:"version"`
		Bundler    Application `xml:"bundler"`
		SignedData []byte      `xml:",innerxml"`
		Signature  string      `xml:"signature"`
	}{
		Version:    "2007-10-10",
		Bundler:    m.Bundler,
		SignedData: signedData.Bytes(),
		Signature:  fmt.Sprintf("%x", signature),
	}

	// Prep an output buffer
	var output bytes.Buffer
	output.WriteString("<?xml version='1.0'?>") // identical to ec2-bundle-image
	if err := xml.NewEncoder(&output).Encode(signedManifest); err != nil {
		return nil, err
	}

	// Success
	return output.Bytes(), nil
}
