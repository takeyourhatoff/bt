package bencode

import (
	"bytes"
	"crypto/sha1"
	"io/ioutil"
	"testing"
)

func TestDecode(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/debian-9.1.0-amd64-netinst.iso.torrent")
	if err != nil {
		t.Fatal(err)
	}
	var m Metainfo
	err = Decode(bytes.NewReader(b), &m)
	if err != nil {
		t.Errorf("%+v", err)
	}
	var buf bytes.Buffer
	err = Encode(&buf, m)
	if err != nil {
		t.Errorf("%+v", err)
	}
	t.Logf("\n%+q\n\t%#v\n%+q\n", b, m, buf.Bytes())
	if bytes.Equal(buf.Bytes(), b) == false {
		t.Error("Encode(Decode(b)) != b")
	}
}

func TestInfohash(t *testing.T) {
	const infohash = "\xfd\x5f\xdf\x21\xae\xf4\x50\x54\x51\x86\x1d\xa9\x7a\xa3\x90\x00\xed\x85\x29\x88"
	b, err := ioutil.ReadFile("testdata/debian-9.1.0-amd64-netinst.iso.torrent")
	if err != nil {
		t.Fatal(err)
	}
	var m Metainfo
	err = Decode(bytes.NewReader(b), &m)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	var buf bytes.Buffer
	Encode(&buf, m.Info)
	t.Logf("infodict:\n%+q\n", buf.Bytes())
	if bytes.Equal(m.Info.Infohash(sha1.New()), []byte(infohash)) == false {
		t.Error("calculated infohash is incorrect")
	}
}
