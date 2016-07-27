package aws_bundle

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"testing"
)

func writeFileToSink(t *testing.T, sink Sink, filename string, contents []byte) {
	if wc, err := sink.WriteBundleFile(filename); err != nil {
		t.Fatalf("unable to WriteBundleFile(%q): %v", filename, err)
	} else if n, err := wc.Write(contents); err != nil || n < len(contents) {
		if err != nil {
			t.Fatalf("error writing to bundle file %q: %v", filename, err)
		} else {
			t.Fatalf("short write while writing to bundle file %q", filename)
		}
	} else if err := wc.Close(); err != nil {
		t.Fatalf("error closting %q", filename)
	}
}

func TestHashingSink(t *testing.T) {
	sink := newAccumulatingSink()
	hs := newHashingSink(sink)

	randomBytes := make([]byte, 65536)
	rand.Read(randomBytes)

	examples := []struct {
		name     string
		contents []byte
		sha1     []byte
	}{
		// SHA-1 test vectors from http://www.di-mgt.com.au/sha_testvectors.html
		{"empty", []byte{}, []byte("\xda\x39\xa3\xee\x5e\x6b\x4b\x0d\x32\x55\xbf\xef\x95\x60\x18\x90\xaf\xd8\x07\x09")},
		{"abc", []byte("abc"), []byte("\xa9\x99\x3e\x36\x47\x06\x81\x6a\xba\x3e\x25\x71\x78\x50\xc2\x6c\x9c\xd0\xd8\x9d")},
		{"longer", []byte("abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq"), []byte("\x84\x98\x3e\x44\x1c\x3b\xd2\x6e\xba\xae\x4a\xa1\xf9\x51\x29\xe5\xe5\x46\x70\xf1")},

		// longer inputs
		{"shakespeare", []byte("To be, or not to be--that is the question:\nWhether 'tis nobler in the mind to suffer\nThe slings and arrows of outrageous fortune\nOr to take arms against a sea of troubles\nAnd by opposing end them. To die, to sleep--\nNo more--and by a sleep to say we end\nThe heartache, and the thousand natural shocks\nThat flesh is heir to. 'Tis a consummation\nDevoutly to be wished. To die, to sleep--\nTo sleep--perchance to dream: ay, there's the rub,\nFor in that sleep of death what dreams may come\nWhen we have shuffled off this mortal coil,\nMust give us pause. There's the respect\nThat makes calamity of so long life.\nFor who would bear the whips and scorns of time,\nTh' oppressor's wrong, the proud man's contumely\nThe pangs of despised love, the law's delay,\nThe insolence of office, and the spurns\nThat patient merit of th' unworthy takes,\nWhen he himself might his quietus make\nWith a bare bodkin? Who would fardels bear,\nTo grunt and sweat under a weary life,\nBut that the dread of something after death,\nThe undiscovered country, from whose bourn\nNo traveller returns, puzzles the will,\nAnd makes us rather bear those ills we have\nThan fly to others that we know not of?\nThus conscience does make cowards of us all,\nAnd thus the native hue of resolution\nIs sicklied o'er with the pale cast of thought,\nAnd enterprise of great pitch and moment\nWith this regard their currents turn awry\nAnd lose the name of action. -- Soft you now,\nThe fair Ophelia! -- Nymph, in thy orisons\nBe all my sins remembered."), nil},
		{"random", randomBytes, nil},
	}

	// write all the examples
	for _, example := range examples {
		writeFileToSink(t, hs, example.name, example.contents)
	}

	// compare them to expected SHA1s
	for i, example := range examples {
		// calculate a SHA1 here if we didn't specify one up-front
		if example.sha1 == nil {
			hash := sha1.New()
			hash.Write(example.contents)
			example.sha1 = hash.Sum(nil)
		}

		// compare
		actual := hs.files[i]
		if actual.filename != example.name {
			t.Errorf("expected a hash for %q, got %q", example.name, actual.filename)
		} else if bytes.Compare(actual.hash, example.sha1) != 0 {
			t.Errorf("expected %q to hash to %v, got %v", example.name, example.sha1, actual.hash)
		}
	}
}
