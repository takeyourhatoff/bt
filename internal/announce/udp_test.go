package annonce

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"net"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/takeyourhatoff/bt/internal/bencode"
)

func readMetainfo(name string) (bencode.Metainfo, error) {
	var m bencode.Metainfo
	f, err := os.Open(name)
	if err != nil {
		return m, err
	}
	err = bencode.Decode(f, &m)
	return m, err
}

func FakeServer(t *testing.T) (net.Addr, <-chan error) {
	c := make(chan error, 1)
	l, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		c <- err
		return nil, c
	}
	go func() {
		defer l.Close()
		defer close(c)
		buf := make([]byte, 512)

		// Connect request
		t.Log("server: listening")
		n, addr, err := l.ReadFrom(buf)
		if err != nil {
			c <- err
			return
		}
		t.Log("server: read packet")
		if n != 16 {
			c <- errors.New("server: expected 16 bytes connect request")
			return
		}
		expected := []byte("\x00\x00\x04\x17\x27\x10\x19\x80" + "\x00\x00\x00\x00")
		if bytes.Equal(buf[:len(expected)], expected) == false {
			c <- errors.Errorf("server: expected %x, got %x", expected, buf[:len(expected)])
			return
		}
		txid := binary.BigEndian.Uint32(buf[len(expected):])
		binary.BigEndian.PutUint32(buf[0:], 0)          // action
		binary.BigEndian.PutUint32(buf[4:], txid)       //txid
		binary.BigEndian.PutUint64(buf[8:], 0xdeadbeef) //connid
		t.Log("server: writing to", addr)
		_, err = l.WriteTo(buf[:16], addr)
		_, err = l.WriteTo(buf[:16], addr)
		_, err = l.WriteTo(buf[:16], addr)
		_, err = l.WriteTo(buf[:16], addr)
		if err != nil {
			c <- err
			return
		}
		t.Log("server: wrote conn response")

		// Announce Request
		n, addr, err = l.ReadFrom(buf)
		if err != nil {
			c <- err
			return
		}
		t.Log("server: read announce")
		if n != 98 {
			c <- errors.New("server: expected 98 bytes announce request")
			return
		}
		connID := binary.BigEndian.Uint64(buf[0:])
		if connID != 0xdeadbeef {
			c <- errors.Errorf("server: expected connid to be %x, was %x", 0xdeadbeef, connID)
			return
		}
		action := binary.BigEndian.Uint32(buf[8:])
		if action != 1 {
			c <- errors.Errorf("server: expected action to be %d, was %d", 1, action)
			return
		}
		txid = binary.BigEndian.Uint32(buf[12:])
		binary.BigEndian.PutUint32(buf[0:], 1)        // action
		binary.BigEndian.PutUint32(buf[4:], txid)     //txid
		binary.BigEndian.PutUint32(buf[8:], 2)        //interval
		copy(buf[20:], net.IPv4(1, 2, 3, 4).To4())    //
		binary.BigEndian.PutUint16(buf[24:], 101)     //port
		copy(buf[26:], net.IPv4(10, 121, 2, 9).To4()) //
		binary.BigEndian.PutUint16(buf[30:], 999)     //port
		t.Log("server: writing to", addr)
		_, err = l.WriteTo(buf[:32], addr)
		if err != nil {
			c <- err
			return
		}
		t.Log("server: wrote response")

	}()
	return l.LocalAddr(), c
}

func TestAnnounceUDP(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	a, err := NewAnnouncer(ctx, net.IPv4(127, 0, 0, 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := readMetainfo("testdata/debian-9.1.0-amd64-netinst.iso.torrent")
	r := AnnounceRequest{
		Event:    EventStarted,
		Infohash: m.Info.Infohash(sha1.New()),
		Left:     m.Info.Length,
		Port:     "6001",
		PeerID:   []byte("CB0000-1234567890123"),
	}
	addr, errc := FakeServer(t)
	if addr == nil {
		t.Error(<-errc)
	}
	peers, next, err := a.Announce(ctx, "udp://"+addr.String()+"/announce", r)
	t.Log("return values of a.Announce ", peers, next, err)
	if err != nil {
		t.Log(<-errc)
		t.Error(err)
	}
}
