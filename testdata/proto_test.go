package testdata

import (
	"sync"
	"testing"
)

type ProtoImpl struct {
	m map[int]int
}

func (p *ProtoImpl) UpdateAndGet(m map[int]int) (map[int]int, error) {
	if m == nil {
		panic("nil map")
	}
	if p.m == nil {
		p.m = make(map[int]int)
	}
	for k, v := range m {
		p.m[k] = v
	}
	return p.m, nil
}

func TestProto(t *testing.T) {
	proto, c := NewProto(&ProtoImpl{})
	count := 100
	var wg sync.WaitGroup
	wg.Add(count)
	for i := range count {
		go func(i int) {
			defer wg.Done()
			for j := range 100 {
				_, _ = proto.UpdateAndGet(map[int]int{j: j + i*count + 1})
			}
		}(i)
	}
	wg.Wait()
	_, err := proto.UpdateAndGet(nil)
	if err == nil || err.Error() != "panic: nil map" {
		t.Fatal(err)
	}
	c()
}
