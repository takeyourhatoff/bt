package bitset

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
)

func TestSet(t *testing.T) {
	f := func(l ascendingInts) bool {
		b := new(Bitset)
		for _, i := range l {
			b.Set(int(i), true)
		}
		for _, i := range l {
			if v := b.Get(int(i)); v == false {
				t.Logf("b.Get(%d) = %v, expected %v", i, v, true)
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestCount(t *testing.T) {
	f := func(l ascendingInts) bool {
		b := new(Bitset)
		for _, i := range l {
			b.Set(int(i), true)
		}
		if count := b.Count(); count != len(l) {
			t.Logf("b.Count() = %d, expected %d", count, len(l))
			return false
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestNthSetBit(t *testing.T) {
	f := func(l ascendingInts) bool {
		b := new(Bitset)
		for _, i := range l {
			b.Set(int(i), true)
		}
		for n, i := range l {
			if nth := b.NthSetBit(n); nth != i {
				t.Logf("b.NthSetBit(%d) = %d, expected %d", n, nth, i)
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestBytes(t *testing.T) {
	f := func(data0 []byte) bool {
		// Get rid of trailing zero bytes
		for len(data0) > 0 && data0[len(data0)-1] == 0 {
			data0 = data0[:len(data0)-1]
		}
		b := new(Bitset)
		b.FromBytes(data0)
		if data1 := b.Bytes(); bytes.Equal(data0, data1) == false {
			t.Logf("b.Bytes() = %v, expected %v", data1, data0)
			return false
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestString(t *testing.T) {
	f := func(l ascendingInts) bool {
		b := new(Bitset)
		for _, i := range l {
			b.Set(int(i), true)
		}
		if s := b.String(); s != fmt.Sprintf("%v", l) {
			t.Logf("b.String() = %v, wanted %v", s, l)
			return false
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestAnd(t *testing.T) {
	f := func(l0, l1 ascendingInts) bool {
		b0 := new(Bitset)
		for _, i := range l0 {
			b0.Set(int(i), true)
		}
		b1 := new(Bitset)
		for _, i := range l0 {
			b1.Set(int(i), true)
		}
		bx := b0.Copy().And(b1)
		for i := 0; i < (len(b0.s)+len(b1.s))*64; i++ {
			if b0.Get(i) && b1.Get(i) {
				if bx.Get(i) == false {
					t.Logf("(b0.Get(%d) && b1.Get(%d)) = true, but bx.Get(%d) = false", i, i, i)
					return false
				}
			} else if bx.Get(i) == true {
				t.Logf("(b0.Get(%d) && b1.Get(%d)) = false, but bx.Get(%d) = true", i, i, i)
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestAndNot(t *testing.T) {
	f := func(l0, l1 ascendingInts) bool {
		b0 := new(Bitset)
		for _, i := range l0 {
			b0.Set(int(i), true)
		}
		b1 := new(Bitset)
		for _, i := range l0 {
			b1.Set(int(i), true)
		}
		bx := b0.Copy().AndNot(b1)
		for i := 0; i < (len(b0.s)+len(b1.s))*64; i++ {
			if b0.Get(i) && !b1.Get(i) {
				if bx.Get(i) == false {
					t.Logf("(b0.Get(%d) && b1.Get(%d)) = true, but bx.Get(%d) = false", i, i, i)
					return false
				}
			} else if bx.Get(i) == true {
				t.Logf("(b0.Get(%d) && b1.Get(%d)) = false, but bx.Get(%d) = true", i, i, i)
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

type ascendingInts []int

func (l ascendingInts) Generate(rand *rand.Rand, size int) reflect.Value {
	l = make([]int, size)
	var x int
	for i := range l {
		x += rand.Intn(100) + 1
		l[i] = x
	}
	return reflect.ValueOf(l)
}
