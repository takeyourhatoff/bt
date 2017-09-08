package bencode

import (
	"bufio"
	"bytes"
	"fmt"
	"hash"
	"io"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Metainfo struct {
	Announce     string    `bencode:"announce"`
	Comment      string    `bencode:"comment,ommitempty"`
	CreationDate time.Time `bencode:"creation date,ommitempty"`
	HTTPSeeds    []string  `bencode:"httpseeds,ommitempty"`
	Info         InfoDict  `bencode:"info"`
}

type InfoDict struct {
	Name        string `bencode:"name"`
	Private     bool   `bencode:"private,ommitempty"`
	PieceLength int64  `bencode:"piece length"`
	RawPieces   []byte `bencode:"pieces"`
	Length      int64  `bencode:"length,ommitempty"`
	Files       []File `bencode:"files,ommitempty"`
}

func (i InfoDict) Pieces() [][]byte {
	pieces := make([][]byte, (len(i.RawPieces)+19)/20)
	for j := range pieces {
		pieces[j] = i.RawPieces[j*20 : (j+1)*20]
	}
	return pieces
}

func (i InfoDict) NumPieces() int {
	return len(i.RawPieces) / 20
}

func (i InfoDict) Infohash(h hash.Hash) []byte {
	var buf bytes.Buffer
	Encode(&buf, i)
	_, _ = buf.WriteTo(h)
	return h.Sum(nil)
}

type File struct {
	Length int64    `bencode:"length"`
	Path   []string `bencode:"path"`
}

type CompactTrackerResponse struct {
	FailureReason string `bencode:"failure reason,ommitempty"`
	Interval      int    `bencode:"interval,ommitempty"`
	Peers         []byte `bencode:"peers,ommitempty"`
}

func (r CompactTrackerResponse) IntervalDuration() time.Duration {
	return time.Duration(r.Interval) * time.Second
}

type TrackerResponse struct {
	FailureReason string `bencode:"failure reason,ommitempty"`
	Interval      int    `bencode:"interval,ommitempty"`
	Peers         []Peer `bencode:"peers,ommitempty"`
}

func (r TrackerResponse) IntervalDuration() time.Duration {
	return time.Duration(r.Interval) * time.Second
}

func (r TrackerResponse) PeerAddrs() ([]net.Addr, error) {
	peers := make([]net.Addr, len(r.Peers))
	for i, p := range r.Peers {
		var err error
		peers[i], err = net.ResolveTCPAddr("tcp", p.String())
		if err != nil {
			return nil, err
		}
	}
	return peers, nil
}

type Peer struct {
	PeerID []byte `bencode:"peer id"`
	IP     string `bencode:"ip"`
	Port   int    `bencode:"port"`
}

func (p Peer) String() string {
	return net.JoinHostPort(p.IP, strconv.Itoa(p.Port))
}

func Encode(w io.Writer, v interface{}) error {
	bw := bufio.NewWriter(w)
	err := encodeT(bw, reflect.Indirect(reflect.ValueOf(v)))
	if err != nil {
		return errors.Wrap(err, "bencoding")
	}
	return errors.Wrap(bw.Flush(), "bencoding")
}

var typeOfBytes = reflect.TypeOf([]byte(nil))

type field struct {
	k string
	v reflect.Value
}

// tagOptions is the string following a comma in a struct field's "json"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

// parseTag splits a struct field's json tag into its name and
// comma-separated options.
func parseTag(tag string) (string, tagOptions) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tagOptions(tag[idx+1:])
	}
	return tag, tagOptions("")
}

// Contains reports whether a comma-separated list of options
// contains a particular substr flag. substr must be surrounded by a
// string boundary or commas.
func (o tagOptions) Contains(optionName string) bool {
	if len(o) == 0 {
		return false
	}
	s := string(o)
	for s != "" {
		var next string
		i := strings.Index(s, ",")
		if i >= 0 {
			s, next = s[:i], s[i+1:]
		}
		if s == optionName {
			return true
		}
		s = next
	}
	return false
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

func encodeT(w *bufio.Writer, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			w.WriteString("i1e")
		} else {
			w.WriteString("i0e")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fmt.Fprintf(w, "i%de", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fmt.Fprintf(w, "i%de", v.Uint())
	case reflect.String:
		_, _ = fmt.Fprintf(w, "%d:%s", v.Len(), v.String())
	case reflect.Slice:
		if v.Type() == typeOfBytes {
			_, _ = fmt.Fprintf(w, "%d:%s", v.Len(), v.Bytes())
		} else {
			_ = w.WriteByte('l')
			for i := 0; i < v.Len(); i++ {
				err := encodeT(w, v.Index(i))
				if err != nil {
					return err
				}
			}
			_ = w.WriteByte('e')
		}
	case reflect.Struct:
		if v.Type() == typeOfTime {
			_, _ = fmt.Fprintf(w, "i%de", v.Interface().(time.Time).Unix())
			return nil
		}
		_ = w.WriteByte('d')
		vt := v.Type()
		var fields []field
		for i := 0; i < v.NumField(); i++ {
			f := vt.Field(i)
			key := f.Name
			if bencode, ok := f.Tag.Lookup("bencode"); ok {
				var opts tagOptions
				key, opts = parseTag(bencode)
				if opts.Contains("ommitempty") && isEmptyValue(v.Field(i)) {
					continue
				}
			}
			fields = append(fields, field{key, v.Field(i)})
		}
		sort.Slice(fields, func(i, j int) bool { return fields[i].k < fields[j].k })
		for _, f := range fields {
			_, _ = fmt.Fprintf(w, "%d:%s", len(f.k), f.k)
			err := encodeT(w, f.v)
			if err != nil {
				return err
			}
		}
		_ = w.WriteByte('e')
	default:
		return errors.Errorf("cannot bencode type %v", v)
	}
	return nil
}

const maxAlloc = 1 << 24

func Decode(r io.Reader, v interface{}) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return errors.New("bencode: attempt to decode into non-pointer")
	}
	br := bufio.NewReader(r)
	err := decodeT(br, reflect.Indirect(val))
	return errors.Wrap(err, "bencoding")
}

func decodeT(r *bufio.Reader, v reflect.Value) error {
	b, err := r.ReadByte()
	if err != nil {
		return errors.WithStack(err)
	}
	switch b {
	case 'i':
		return decodeInt(r, v)
	case 'l':
		return decodeList(r, v)
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		_ = r.UnreadByte()
		return decodeString(r, v)
	case 'd':
		return decodeDict(r, v)
	default:
		return errors.Errorf("unexpected %q", b)
	}
}

var typeOfTime = reflect.TypeOf(time.Time{})

func decodeInt(r *bufio.Reader, v reflect.Value) error {
	switch {
	case v.IsValid() == false:
		var i int64
		_, err := fmt.Fscanf(r, "%de", &i)
		if err != nil {
			return errors.Wrap(err, "scanning int")
		}
	case v.Type() == typeOfTime:
		var i int64
		_, err := fmt.Fscanf(r, "%de", &i)
		if err != nil {
			return errors.Wrap(err, "scanning int")
		}
		v.Set(reflect.ValueOf(time.Unix(i, 0)))
	case v.Kind() == reflect.Bool:
		var b byte
		_, err := fmt.Fscanf(r, "%ce", &b)
		if err != nil {
			return errors.Wrap(err, "scanning int")
		}
		switch b {
		case '0':
			v.SetBool(false)
		case '1':
			v.SetBool(true)
		default:
			return errors.Errorf("found '%c', expected '0' or '1' when scanning bool", b)
		}
	case v.Kind() >= reflect.Int && v.Kind() <= reflect.Int64:
		var i int64
		_, err := fmt.Fscanf(r, "%de", &i)
		if err != nil {
			return errors.Wrap(err, "scanning int")
		}
		v.SetInt(i)
	case v.Kind() >= reflect.Uint && v.Kind() <= reflect.Uint64:
		var i uint64
		_, err := fmt.Fscanf(r, "%de", &i)
		if err != nil {
			return errors.Wrap(err, "scanning int")
		}
		v.SetUint(i)
	default:
		return errors.Errorf("cannot store integer in %v", v.Kind())
	}
	return nil
}
func decodeString(r io.Reader, v reflect.Value) error {
	var n int
	_, err := fmt.Fscanf(r, "%d:", &n)
	if err != nil {
		return errors.Wrap(err, "scanning string length")
	}
	if n < 0 || n > maxAlloc {
		return errors.New("invalid string length")
	}
	buf := make([]byte, n)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return errors.Wrap(err, "reading string")
	}
	switch {
	case v.IsValid() == false:
	case v.Type() == typeOfBytes:
		v.SetBytes(buf)
	case v.Kind() == reflect.String:
		v.SetString(string(buf))
	default:
		return errors.Errorf("cannot store string in %v", v.Type())
	}
	return nil
}

func decodeList(r *bufio.Reader, v reflect.Value) error {
	if v.IsValid() && v.Kind() != reflect.Slice {
		return errors.Errorf("cannot store list in %v", v.Kind())
	}
	if v.IsValid() {
		v.Set(v.Slice(0, 0))
	}
	for {
		b, err := r.ReadByte()
		if err != nil {
			return errors.WithStack(err)
		}
		if b == 'e' {
			return nil
		}
		_ = r.UnreadByte()
		if v.IsValid() {
			elem := reflect.Indirect(reflect.New(v.Type().Elem()))
			err = decodeT(r, elem)
			v.Set(reflect.Append(v, elem))
		} else {
			err = decodeT(r, reflect.Value{})
		}
		if err != nil {
			return err
		}
	}
}

func decodeDict(r *bufio.Reader, v reflect.Value) error {
	//BUG: Will not return an error when leading spaces or zeros are present in an int
	if v.IsValid() && v.Kind() != reflect.Struct {
		return errors.Errorf("cannot decode dictionary into %v", v.Type())
	}
	var lastName []byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			return errors.WithStack(err)
		}
		if b == 'e' {
			return nil
		}
		_ = r.UnreadByte()
		var n int
		_, err = fmt.Fscanf(r, "%d:", &n)
		if err != nil {
			return errors.Wrap(err, "scanning dict key length prefix")
		}
		if n < 0 || n > maxAlloc {
			return errors.New("invalid dict key length")
		}
		name := make([]byte, n)
		_, err = io.ReadFull(r, name)
		if err != nil {
			return errors.Wrap(err, "scanning dict key")
		}
		if bytes.Compare(lastName, name) == 1 {
			return errors.Errorf("%q appeared after %q in dict despite being lexiographically smaller", name, lastName)
		}
		lastName = name
		if v.IsValid() {
			i := structIndexFromName(v.Type(), string(name))
			if i >= 0 {
				err = decodeT(r, v.Field(i))
			} else {
				err = decodeT(r, reflect.Value{})
			}
			if err != nil {
				return err
			}
		}
	}
}

func structIndexFromName(t reflect.Type, name string) int {
	for i := 0; i < t.NumField(); i++ {
		tf := t.Field(i)
		fname := tf.Name
		if bencode, ok := tf.Tag.Lookup("bencode"); ok {
			fname, _ = parseTag(bencode)
		}
		if fname == name {
			return i
		}
	}
	return -1
}
