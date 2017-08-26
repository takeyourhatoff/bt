package bitset

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func TestString(t *testing.T) {
	ss := []int{1, 2, 3, 10, 100, 101, 123}
	s := new(Bitset)
	for _, n := range ss {
		s.Set(n, true)
	}
	if s.String() != fmt.Sprint(ss) {
		t.Errorf("s.String() = %q, wanted %q.", s.String(), fmt.Sprint(ss))
	}
}

func TestMarshalBinary(t *testing.T) {
	ss := []int{1, 2, 3, 10, 100, 101, 123, 150}
	s := new(Bitset)
	for _, n := range ss {
		s.Set(n, true)
	}
	b, _ := s.MarshalBinary()
	s1 := new(Bitset)
	s1.UnmarshalBinary(b)
	if !reflect.DeepEqual(s, s1) {
		t.Errorf("s = %v, s1 = %v", s, s1)
	}
	var buf bytes.Buffer
	for _, b0 := range b {
		fmt.Fprintf(&buf, "%08b", b0)
	}
	t.Log(buf.String())
	t.Log(s)
}
