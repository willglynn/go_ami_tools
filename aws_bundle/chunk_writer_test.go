package aws_bundle

import (
	"bytes"
	"io"
	"testing"
)

type accumulatingSink struct {
	files map[string]*bytes.Buffer
}

func (as *accumulatingSink) WriteBundleFile(filename string) (io.WriteCloser, error) {
	buffer := &bytes.Buffer{}

	if as.files[filename] != nil {
		panic("attempt to write duplicate file")
	}
	as.files[filename] = buffer

	return &nopWriteCloser{buffer}, nil
}

func newAccumulatingSink() *accumulatingSink {
	return &accumulatingSink{
		files: make(map[string]*bytes.Buffer),
	}
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error {
	return nil
}

func testChunkWriter(t *testing.T, writeSize int) {
	sink := newAccumulatingSink()
	cw := newChunkWriter(sink, "test", 100)

	testInput := []byte(`Lorem ipsum dolor sit amet, consectetur adipiscing elit. Praesent felis leo, rhoncus id aliquam ac, volutpat eu magna. Integer id tortor nulla. Donec vitae consequat lacus. Maecenas porta, elit quis dapibus elementum, eros nunc suscipit dui, vel tempus diam nisi quis elit. Suspendisse diam nisl, tempor eu lacinia nec, convallis eu tortor. Praesent at enim ornare, sagittis justo id, tristique nibh. Donec in faucibus velit, a congue metus. Donec sed semper magna. Cras commodo, massa quis pretium vestibulum, ligula neque sollicitudin nulla, ac sagittis lectus massa at ex. Sed sed eros eget mi sollicitudin mollis vel maximus nibh. Cras bibendum leo congue vulputate condimentum.`)

	toWrite := testInput
	for len(toWrite) > 0 {
		thisWrite := writeSize
		if thisWrite > len(toWrite) {
			thisWrite = len(toWrite)
		}
		now, later := toWrite[0:thisWrite], toWrite[thisWrite:]
		toWrite = later

		n, err := cw.Write(now)
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}
		if n != len(now) {
			t.Errorf("wrote %d bytes instead of %d", n, len(now))
		}
	}

	if err := cw.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	expectedFiles := []struct {
		name     string
		expected []byte
	}{
		{"test.part.0", testInput[0:100]},
		{"test.part.1", testInput[100:200]},
		{"test.part.2", testInput[200:300]},
		{"test.part.3", testInput[300:400]},
		{"test.part.4", testInput[400:500]},
		{"test.part.5", testInput[500:600]},
		{"test.part.6", testInput[600:len(testInput)]},
	}

	// compare the contents of the sink
	for _, file := range expectedFiles {
		actual := sink.files[file.name]
		if actual == nil {
			t.Errorf("expected file %q, got none", file.name)
		} else if bytes.Compare(file.expected, actual.Bytes()) != 0 {
			t.Errorf("file %q had different contents than expected", file.name)
		}
	}
	if len(sink.files) != len(expectedFiles) {
		t.Errorf("expected %d files, got %d", len(expectedFiles), len(sink.files))
	}
}

func TestChunkWriter(t *testing.T) {
	// run the same test using a variety of Write() sizes
	for _, size := range []int{1024, 101, 100, 99, 101, 51, 50, 49, 25, 24, 3, 2, 1} {
		testChunkWriter(t, size)
	}
}
