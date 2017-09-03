package bitset

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/bits"
)

const maxUint = 1<<bits.UintSize - 1

type Bitset struct {
	s []uint
}

func (s *Bitset) Set(i int, v bool) {
	if i < 0 {
		panic("i < 0")
	}
	if v {
		s.setTrue(i)
	} else {
		s.setFalse(i)
	}
}

func (s *Bitset) setTrue(i int) {
	for j := len(s.s); j <= i/bits.UintSize; j++ {
		s.s = append(s.s, 0)
	}
	s.s[i/bits.UintSize] |= 1 << uint(i%bits.UintSize)
}

func (s *Bitset) setFalse(i int) {
	if i/bits.UintSize < len(s.s) {
		s.s[i/bits.UintSize] &^= 1 << uint(i%bits.UintSize)
	}
}

func (s *Bitset) Get(i int) bool {
	if i < 0 {
		panic("i < 0")
	}
	if i/bits.UintSize >= len(s.s) {
		return false
	}
	mask := uint(1 << uint(i%bits.UintSize))
	return s.s[i/bits.UintSize]&mask != 0
}

func (s *Bitset) And(ss *Bitset) *Bitset {
	n := min(len(s.s), len(ss.s))
	for i := 0; i < n; i++ {
		s.s[i] &= ss.s[i]
	}
	for i := n; i < len(s.s); i++ {
		s.s[i] = 0
	}
	return s
}

func (s *Bitset) AndNot(ss *Bitset) *Bitset {
	n := min(len(s.s), len(ss.s))
	for i := 0; i < n; i++ {
		s.s[i] &^= ss.s[i]
	}
	return s
}

func (s *Bitset) Count() int {
	var n int
	for i := range s.s {
		n += bits.OnesCount(s.s[i])
	}
	return n
}

func (s *Bitset) NthSetBit(n int) int {
	if n < 0 {
		panic("n < 0")
	}
	for i := s.NextSetBitAfter(0); i >= 0; i = s.NextSetBitAfter(i + 1) {
		if n == 0 {
			return i
		}
		n--
	}
	return -1
}

func (s *Bitset) NextSetBitAfter(i int) int {
	if i < 0 {
		panic("i < 0")
	}
	mask := uint(maxUint) >> (bits.UintSize - uint(i)%bits.UintSize)
	for j := i / bits.UintSize; j < len(s.s); j++ {
		word := s.s[j] &^ mask
		mask = 0
		if word != 0 {
			return j*bits.UintSize + bits.TrailingZeros(word)
		}
	}
	return -1
}

func (s *Bitset) NextUnsetBitAfter(i int) int {
	if i < 0 {
		panic("i < 0")
	}
	mask := uint(maxUint) >> (bits.UintSize - uint(i)%bits.UintSize)
	for j := i / bits.UintSize; j < len(s.s); j++ {
		word := ^(s.s[j] &^ mask)
		mask = 0
		if word != 0 {
			return j*bits.UintSize + bits.TrailingZeros(word)
		}
	}
	return len(s.s) * bits.UintSize
}

func (s *Bitset) Copy() *Bitset {
	ss := new(Bitset)
	ss.s = make([]uint, len(s.s))
	copy(ss.s, s.s)
	return ss
}

func (s *Bitset) String() string {
	var buf bytes.Buffer
	buf.WriteRune('[')
	first := true
	for i := s.NextSetBitAfter(0); i >= 0; i = s.NextSetBitAfter(i + 1) {
		if !first {
			buf.WriteRune(' ')
		}
		fmt.Fprintf(&buf, "%d", i)
		first = false
	}
	buf.WriteRune(']')
	return buf.String()
}

func (s *Bitset) Bytes() []byte {
	const r = bits.UintSize / 8
	b := make([]byte, len(s.s)*r)
	b0 := b
	for _, v := range s.s {
		v = bits.Reverse(v)
		switch bits.UintSize {
		case 32:
			binary.BigEndian.PutUint32(b, uint32(v))
		case 64:
			binary.BigEndian.PutUint64(b, uint64(v))
		default:
			panic("uint is not 32 or 64 bits long")
		}
		b = b[r:]
	}
	for len(b0) > 0 && b0[len(b0)-1] == 0 {
		b0 = b0[:len(b0)-1]
	}
	return b0
}

func (s *Bitset) FromBytes(data []byte) *Bitset {
	const r = bits.UintSize / 8
	if len(data) == 0 {
		s.s = nil
	}
	for len(data)%r != 0 {
		data = append(data, 0)
	}
	s.s = make([]uint, len(data)/r)
	for i := range s.s {
		switch bits.UintSize {
		case 32:
			s.s[i] = uint(binary.BigEndian.Uint32(data))
		case 64:
			s.s[i] = uint(binary.BigEndian.Uint64(data))
		default:
			panic("uint is not 32 or 64 bits long")
		}
		s.s[i] = bits.Reverse(s.s[i])
		data = data[r:]
	}
	return s
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}
