package aws_bundle

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"io"
)

type aesCbcWriter struct {
	w      io.Writer
	buffer []byte // accumulates writes of partial blocks
	cbc    cipher.BlockMode
}

func newAes128CbcWriter(w io.Writer, key []byte, iv []byte) (io.WriteCloser, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	cbc := cipher.NewCBCEncrypter(c, iv)

	return &aesCbcWriter{
		w:      w,
		buffer: make([]byte, 0, 16),
		cbc:    cbc,
	}, nil
}

func (a *aesCbcWriter) Write(p []byte) (n int, err error) {
	if a.cbc == nil {
		return 0, errors.New("AES-128-CBC writer is already closed")
	}

	// Do we have any buffered plaintext?
	if len(a.buffer) > 0 {
		// Peel off some bytes to make a.buffer a full block, and reduce p accordingly
		x := 16 - len(a.buffer)
		if x > len(p) {
			x = len(p)
		}
		a.buffer = append(a.buffer, p[:x]...)
		p = p[x:]

		// Do we have a full block?
		if len(a.buffer) == 16 {
			// Write the full block
			_, err = a.writeBlocks(a.buffer)

			// Empty the buffer
			a.buffer = a.buffer[:0]

			// Remember how many of the caller's bytes we wrote
			n += x

			// Bail on failure
			if err != nil {
				return n, err
			}
		}
	}

	// We now have either a) nothing in our buffer or b) nothing in p
	// Write full blocks from p
	n2, err := a.writeBlocks(p)
	n += n2

	// Buffer anything remaining in p
	if len(p) > n2 {
		// Copy it to the buffer
		remainder := p[n2:]
		a.buffer = append(a.buffer, remainder...)
		n += len(remainder)
	}

	// Done!
	return n, err
}

// Write as many full blocks as possible, returning the number of bytes written.
// Since it only writes full blocks, a short write does not indicate an error.
func (a *aesCbcWriter) writeBlocks(plaintext []byte) (n int, err error) {
	// calculate the number of bytes we will write
	// (integer math rounds down, so this does what we want)
	bytes := (len(plaintext) / 16) * 16

	// encrypt
	ciphertext := make([]byte, bytes)
	a.cbc.CryptBlocks(ciphertext, plaintext[:bytes])

	// write the encrypted data
	return a.w.Write(ciphertext)
}

func (a *aesCbcWriter) Close() error {
	if a.cbc == nil {
		return errors.New("AES-128-CBC writer is already closed")
	}

	// Finish encrypting. That means: flush the buffer.
	//
	// ec2-ami-tools encrypts via:
	//   `#{openssl} enc -e -aes-128-cbc -K #{key} -iv #{iv}`
	//
	// According to the docs, `openssl enc` uses PKCS#5 padding. Unfortunately
	// PKCS#5 is defined only for 8-byte block sizes, so they actually mean it
	// pads using PKCS#7.

	// Add padding to the buffer until it's a whole block
	plaintextBytes := len(a.buffer)
	paddingByte := byte(16 - plaintextBytes)
	for len(a.buffer) < 16 {
		a.buffer = append(a.buffer, paddingByte)
	}

	// Encrypt and write the final block
	_, err := a.writeBlocks(a.buffer)
	if err != nil {
		return err
	}

	a.cbc = nil
	return nil
}
