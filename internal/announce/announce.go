package annonce

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	neturl "net/url"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/takeyourhatoff/bt/internal/bencode"
)

type EventType int

const (
	EventScheduled EventType = 0
	EventCompleted EventType = 1
	EventStarted   EventType = 2
	EventStopped   EventType = 3
)

func (t EventType) String() string {
	switch t {
	case EventScheduled:
		return ""
	case EventCompleted:
		return "completed"
	case EventStarted:
		return "started"
	case EventStopped:
		return "stopped"
	default:
		return "!!!INVALID EVENT!!!"
	}
}

type AnnounceRequest struct {
	Infohash []byte
	PeerID   []byte
	Port     string
	Up, Down int64
	Left     int64
	Event    EventType
	NumWant  int
}

type Announcer struct {
	client      *http.Client
	conn        net.PacketConn
	resp        map[uint32]chan udpResponse // map of transaction id's to receivers for udp responses
	respMu      sync.Mutex                  // protects resp
	connIDCache *ttlCache
	ipv6        bool
	err         error
	errMu       sync.Mutex
	logger      *log.Logger
}

type udpResponse struct {
	action actionType
	body   io.Reader
}

func NewAnnouncer(ctx context.Context, ip net.IP, logger *log.Logger) (*Announcer, error) {
	tcpAddr := &net.TCPAddr{
		IP:   ip,
		Port: 0,
	}
	// copy http.DefaultClient's settings. Ensure dialing from same IP as the one we are given and ignore happy eyeballs
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			LocalAddr: tcpAddr,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
	}
	udpAddr := net.UDPAddr{
		IP:   ip,
		Port: 0,
	}
	conn, err := net.ListenPacket(udpAddr.Network(), udpAddr.String())
	if err != nil {
		return nil, err
	}
	a := &Announcer{
		client: client,
		conn:   conn,
		resp:   make(map[uint32]chan udpResponse),
		logger: logger,
		ipv6:   ip.To4() == nil,
	}
	a.connIDCache = newCache(a.getConnID)
	go a.listen(ctx)
	return a, nil
}

func (a *Announcer) getErr() error {
	a.errMu.Lock()
	defer a.errMu.Unlock()
	return a.err
}

func (a *Announcer) setErr(err error) {
	a.errMu.Lock()
	defer a.errMu.Unlock()
	if a.err == nil {
		a.err = err
	}
}

func (a *Announcer) Announce(ctx context.Context, url string, r AnnounceRequest) (peers []net.Addr, nextAnnounce time.Time, err error) {
	u, err := neturl.Parse(url)
	if err != nil {
		return
	}
	switch u.Scheme {
	case "http", "https":
		return a.announceHTTP(ctx, u, r)
	case "udp":
		return a.announceUDP(ctx, u, r)
	default:
		err = errors.Errorf("unrecognised scheme %q", u.Scheme)
		return
	}
}

func (a *Announcer) announceHTTP(ctx context.Context, url *neturl.URL, r AnnounceRequest) (peers []net.Addr, nextAnnounce time.Time, err error) {
	const userAgent = "cbv0"
	q := url.Query()
	q.Set("info_hash", string(r.Infohash))
	q.Set("peer_id", string(r.PeerID))
	q.Set("port", r.Port)
	q.Set("uploaded", strconv.FormatInt(r.Up, 10))
	q.Set("downloaded", strconv.FormatInt(r.Down, 10))
	q.Set("left", strconv.FormatInt(r.Left, 10))
	q.Set("numwant", strconv.FormatInt(r.Left, 10))
	q.Set("compact", "1")
	if r.Event != EventScheduled {
		q.Set("event", r.Event.String())
	}
	url.RawQuery = q.Encode()
	if a.logger != nil {
		a.logger.Printf("announce: %v\n", url.String())
	}
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", userAgent)
	resp, err := a.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var mresp bencode.CompactTrackerResponse
	err = bencode.Decode(resp.Body, &mresp)
	if err != nil {
		// Maby tracker sent non-compact response
		var mresp bencode.TrackerResponse
		err = bencode.Decode(resp.Body, &mresp)
		if err != nil {
			// Maby not
			return
		}
		if mresp.FailureReason != "" {
			err = errors.Errorf("tracker error: %s", mresp.FailureReason)
		}
		peers, err = mresp.PeerAddrs()
		if err != nil {
			return
		}
		nextAnnounce = time.Now().Add(mresp.IntervalDuration())
	} else {
		r := bytes.NewReader(mresp.Peers)
		peers, err = readPeers(r, a.ipv6)
		nextAnnounce = time.Now().Add(mresp.IntervalDuration())
	}
	return
}
