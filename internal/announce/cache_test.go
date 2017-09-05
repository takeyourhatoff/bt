package annonce

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
)

func TestCache(t *testing.T) {
	var once bool
	f := func(_ context.Context, _ key) (item, error) {
		if once {
			return item{}, errors.New("f called twice")
		}
		once = true
		return item{
			exp: time.Now().Add(1 * time.Minute),
			v:   nil,
		}, nil
	}
	c := newCache(f)
	_, err := c.get(context.Background(), nil)
	if err != nil {
		t.Errorf("c.get() = %v, expected nil", err)
	}
	_, err = c.get(context.Background(), nil)
	if err != nil {
		t.Errorf("c.get() = %v, expected nil", err)
	}
}

/*
func TestTTL(t *testing.T) {
	future := time.Now().Add(1 * time.Minute)
	past := time.Now().Add(-1 * time.Minute)
	f := func(_ context.Context, k key) (item, error) {

	}
}
*/
