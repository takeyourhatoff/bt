package annonce

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"io"
	"io/ioutil"
	"net"
	neturl "net/url"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

func (a *Announcer) listen(ctx context.Context) {
	const readDeadline = 1 * time.Second
	const maxPacketSize = 512
	for ctx.Err() == nil {
		err := a.conn.SetReadDeadline(time.Now().Add(readDeadline))
		if err != nil {
			a.setErr(err)
			return
		}
		buf := make([]byte, maxPacketSize)
		n, addr, err := a.conn.ReadFrom(buf)
		if err, ok := err.(net.Error); ok && err.Temporary() {
			time.Sleep(1 * time.Second)
			continue
		} else if err != nil {
			a.setErr(err)
			return
		}
		r := bytes.NewReader(buf[:n])
		type header struct {
			Action actionType
			ConnID uint32
		}
		var h header
		err = binary.Read(r, binary.BigEndian, &h)
		if err != nil {
			continue
		}
		if a.logger != nil {
			a.logger.Printf("receiving packet from %v:\n\t%#v\n", addr, h)
		}
		a.respMu.Lock()
		if c, ok := a.resp[h.ConnID]; ok {
			select {
			case c <- udpResponse{
				action: h.Action,
				body:   r,
			}:
			default:
			}
		}
		a.respMu.Unlock()
	}
}

func (a *Announcer) getConnID(ctx context.Context, k key) (item, error) {
	const ttl = 1 * time.Minute
	addr, err := net.ResolveUDPAddr("udp", k.(string))
	if err != nil {
		return item{}, err
	}
	r, err := a.sendRequest(ctx, addr, actionConnect, nil)
	if err != nil {
		return item{}, err
	}
	var connID uint64
	err = binary.Read(r, binary.BigEndian, &connID)
	if err != nil {
		return item{}, err
	}
	i := item{
		exp: time.Now().Add(ttl),
		v:   connID,
	}
	return i, nil
}

type actionType uint32

const (
	actionConnect  actionType = 0
	actionAnnounce actionType = 1
	actionScrape   actionType = 2
	actionError    actionType = 3
)

type timeoutError struct {
	error
}

func (e timeoutError) Timeout() bool   { return true }
func (e timeoutError) Temporary() bool { return true }

var _ net.Error = timeoutError{}

func (a *Announcer) getTxid() (txid uint32, c chan udpResponse, free func()) {
	a.respMu.Lock()
	defer a.respMu.Unlock()
	c = make(chan udpResponse)
	for {
		err := binary.Read(rand.Reader, binary.BigEndian, &txid)
		if err != nil {
			panic("rand.Reader.Read failed")
		}
		if _, ok := a.resp[txid]; ok {
			continue
		}
		a.resp[txid] = c
		break
	}
	return txid, c, func() {
		a.respMu.Lock()
		defer a.respMu.Unlock()
		delete(a.resp, txid)
	}
}

func (a *Announcer) sendRequest(ctx context.Context, addr net.Addr, action actionType, req []byte) (io.Reader, error) {
	const protocolID = 0x41727101980
	type reqHeader struct {
		ConnectionID  uint64
		Action        actionType
		TransactionID uint32
	}
	head := reqHeader{
		Action: action,
	}
	var c chan udpResponse
	var free func()
	head.TransactionID, c, free = a.getTxid()
	defer free()

	var buf bytes.Buffer
	var err error
	for n := uint(0); n <= 8; n++ {
		if action == actionConnect {
			head.ConnectionID = protocolID
		} else {
			head.ConnectionID, err = a.connIDForHost(ctx, addr)
			if err != nil {
				return nil, err
			}
		}
		if a.logger != nil {
			a.logger.Printf("sending packet to %v:\n\t%#v\n", addr, head)
		}
		binary.Write(&buf, binary.BigEndian, head)
		buf.Write(req)

		_, err := a.conn.WriteTo(buf.Bytes(), addr)
		if err != nil {
			return nil, err
		}
		buf.Reset()

		if err := a.getErr(); err != nil {
			return nil, err
		}
		select {
		case resp := <-c:
			if resp.action == actionError { // error
				errMsg, err := ioutil.ReadAll(resp.body)
				if err != nil {
					return nil, err
				}
				return nil, errors.Errorf("tracker error: %q", errMsg)
			} else {
				return resp.body, nil
			}
		case <-time.After(15 * 1 << n * time.Second):
			continue // resend packet
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, timeoutError{errors.New("announce: timeout waiting for tracker response")}
}

func (a *Announcer) connIDForHost(ctx context.Context, addr net.Addr) (uint64, error) {
	i, err := a.connIDCache.get(ctx, addr.String())
	if err != nil {
		return 0, err
	}
	return i.v.(uint64), nil
}

func (a *Announcer) announceUDP(ctx context.Context, url *neturl.URL, r AnnounceRequest) (peers []net.Addr, nextAnnounce time.Time, err error) {
	addr, err := net.ResolveUDPAddr("udp4", url.Host) //TODO: ipv6
	if err != nil {
		return
	}
	port, err := strconv.Atoi(r.Port)
	if err != nil {
		return
	}
	type request struct {
		Infohash   [20]byte
		PeerID     [20]byte
		Downloaded uint64
		Left       uint64
		Uploaded   uint64
		Event      uint32
		IP         [4]byte
		Key        uint32
		NumWant    uint32
		Port       uint16
	}
	req := request{
		Downloaded: uint64(r.Down),
		Left:       uint64(r.Left),
		Uploaded:   uint64(r.Up),
		Event:      uint32(r.Event),
		Key:        0, //TODO: if announcing to ipv6 and ipv4 at the same time
		NumWant:    uint32(r.NumWant),
		Port:       uint16(port),
	}
	copy(req.Infohash[:], r.Infohash)
	copy(req.PeerID[:], r.PeerID)

	var buf bytes.Buffer
	err = binary.Write(&buf, binary.BigEndian, req)
	if err != nil {
		return
	}
	rr, err := a.sendRequest(ctx, addr, actionAnnounce, buf.Bytes())
	if err != nil {
		return
	}
	type response struct {
		Interval, Leechers, Seeders uint32
	}
	var resp response
	err = binary.Read(rr, binary.BigEndian, &resp)
	if err != nil {
		return
	}
	peers, err = readPeers(rr, a.ipv6)
	if err != nil {
		return
	}
	interval := time.Second * time.Duration(resp.Interval)
	return peers, time.Now().Add(interval), nil
}

func readPeers(r io.Reader, ipv6 bool) ([]net.Addr, error) {
	if ipv6 {
		return readPeers6(r)
	}
	return readPeers4(r)
}

func readPeers4(r io.Reader) ([]net.Addr, error) {
	var peers []net.Addr
	for {
		type peer struct {
			IP   [net.IPv4len]byte
			Port uint16
		}
		var p peer
		err := binary.Read(r, binary.BigEndian, &p)
		if err == io.EOF {
			return peers, nil
		}
		if err != nil {
			return nil, err
		}
		pAddr := &net.TCPAddr{
			IP:   p.IP[:],
			Port: int(p.Port),
		}
		peers = append(peers, pAddr)
	}
}

func readPeers6(r io.Reader) ([]net.Addr, error) {
	var peers []net.Addr
	for {
		type peer struct {
			IP   [net.IPv6len]byte
			Port uint16
		}
		var p peer
		err := binary.Read(r, binary.BigEndian, &p)
		if err == io.EOF {
			return peers, nil
		}
		if err != nil {
			return nil, err
		}
		pAddr := &net.TCPAddr{
			IP:   p.IP[:],
			Port: int(p.Port),
		}
		peers = append(peers, pAddr)
	}
}
