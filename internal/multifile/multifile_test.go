package multifile

import (
	"crypto/sha1"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/takeyourhatoff/bt/internal/bencode"
	"github.com/takeyourhatoff/bt/internal/iox"
)

func readMetainfo(name string) (bencode.Metainfo, error) {
	var m bencode.Metainfo
	f, err := os.Open(name)
	defer f.Close()
	if err != nil {
		return m, err
	}
	err = bencode.Decode(f, &m)
	if err != nil {
		return m, err
	}
	return m, nil
}

func TestRoundTrip(t *testing.T) {
	m := bencode.Metainfo{
		Info: bencode.InfoDict{
			Name: "test_torrent",
			Files: []bencode.File{
				{Length: 0,
					Path: []string{"zero"}},
				{Length: 1,
					Path: []string{"one"}},
				{Length: 5,
					Path: []string{"5"}},
				{Length: 0,
					Path: []string{"zeroInMiddle"}},
				{Length: 42,
					Path: []string{"42"}},
				{Length: 1,
					Path: []string{"dir/one"}},
				{Length: 0,
					Path: []string{"dir/zero"}},
			},
		},
	}
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	f0, err := Create(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := f0.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	// Filling f0 with data and recording hash
	var r io.Reader
	r = &repeatReader{
		b: []byte("TheQuickBrownFoxJumpedOverTheLazyDog"),
	}
	r = io.LimitReader(r, f0.Size())
	h0 := sha1.New()
	r = io.TeeReader(r, h0)
	w := iox.NewSectionWriter(f0, 0)
	_, err = io.Copy(w, r)
	if err != nil {
		t.Fatal(err)
	}

	// Reading data back
	f1, err := Open(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := f1.Close()
		if err != nil {
			t.Error(err)
		}
	}()
	r = io.NewSectionReader(f0, 0, f0.Size())
	r = iox.NewChecksumReader(r, sha1.New(), h0.Sum(nil))
	_, err = io.Copy(ioutil.Discard, r)
	if err != nil {
		t.Error(err)
	}
}

type repeatReader struct {
	b   []byte
	off int
}

func (r *repeatReader) Read(p []byte) (n int, err error) {
	n = copy(p, r.b[r.off:])
	r.off = (r.off + n) % len(r.b)
	return n, nil
}
