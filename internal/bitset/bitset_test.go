package bitset

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"testing"
	"testing/quick"
)

// NextAfter can be used to iterate over the elements of the set.
func ExampleBitset_NextAfter() {
	s := new(Bitset)
	s.Add(2)
	s.Add(42)
	s.Add(13)
	for i := s.NextAfter(0); i >= 0; i = s.NextAfter(i + 1) {
		fmt.Println(i)
	}
	// Output:
	// 2
	// 13
	// 42
}

func ExampleBitset_String() {
	s := new(Bitset)
	s.Add(2)
	s.Add(42)
	s.Add(13)
	fmt.Println(s)
	// Output: [2 13 42]
}

func TestAdd(t *testing.T) {
	f := func(l ascendingInts) bool {
		b := new(Bitset)
		for _, i := range l {
			b.Add(int(i))
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

func TestAdd_Panic(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("b.Add(-1) did not panic")
		} else if err, ok := r.(runtime.Error); ok {
			t.Error(err)
		}
	}()
	new(Bitset).Add(-1)
}

func TestMax(t *testing.T) {
	f := func(l ascendingInts) bool {
		b := new(Bitset)
		for _, i := range l {
			b.Add(int(i))
		}
		if b.Get(1000) == false {
			// if we are not expecting it, add and remove a very large value to test b.Max() ignores trailing zero words.
			b.Add(1000)
			b.Remove(1000)
		}
		max := b.Max()
		if len(l) == 0 {
			if max == -1 {
				return true
			} else {
				t.Logf("b.Max() = %v, expected -1", max)
				return false
			}
		}
		if lMax := l[len(l)-1]; max != lMax {
			t.Logf("b.Max() = %v, expected %v", max, lMax)
			return false
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
			b.Add(int(i))
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

func TestNextAfter(t *testing.T) {
	f := func(l ascendingInts) bool {
		b := new(Bitset)
		for _, i := range l {
			b.Add(int(i))
		}
		var n int
		var oldi int
		for i := b.NextAfter(0); i >= 0; i = b.NextAfter(i + 1) {
			if l[n] != i {
				t.Logf("b.NextAfter(%d) = %d, expected %d", oldi, i, l[n])
				return false
			}
			oldi = i
			n++
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
			b.Add(int(i))
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
			b0.Add(int(i))
		}
		b1 := new(Bitset)
		for _, i := range l1 {
			b1.Add(int(i))
		}
		bx := b0.Copy()
		bx.And(b1)
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
			b0.Add(int(i))
		}
		b1 := new(Bitset)
		for _, i := range l1 {
			b1.Add(int(i))
		}
		bx := b0.Copy()
		bx.AndNot(b1)
		for i := 0; i < (len(b0.s)+len(b1.s))*64; i++ {
			if b0.Get(i) && !b1.Get(i) {
				if bx.Get(i) == false {
					t.Logf("(b0.Get(%d) && !b1.Get(%d)) = true, but bx.Get(%d) = false", i, i, i)
					return false
				}
			} else if bx.Get(i) == true {
				t.Logf("(b0.Get(%d) && !b1.Get(%d)) = false, but bx.Get(%d) = true", i, i, i)
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
	l = make([]int, rand.Intn(size))
	var x int
	for i := range l {
		x += rand.Intn(100) + 1
		l[i] = x
	}
	return reflect.ValueOf(l)
}
