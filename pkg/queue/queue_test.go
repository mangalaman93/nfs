package queue

import (
	"testing"
	"time"
)

/* Example benchmark results [Intel® Core™ i7-3612QM CPU @ 2.10GHz × 8] Ubuntu 15.10, 8GB
PASS
BenchmarkWordAdd-8   	100000000	        23.0 ns/op	      21 B/op	       0 allocs/op
BenchmarkWordRemove-8	50000000	        28.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkTimeAdd-8   	10000000	       246 ns/op	      58 B/op	       1 allocs/op
BenchmarkTimeRemove-8	20000000	       128 ns/op	      32 B/op	       1 allocs/op
ok  	github.com/mangalaman93/nfs/pkg/queue	10.305s
*/

func TestWordQueue(t *testing.T) {
	q := NewWordQueue(2)

	for j := int64(0); j < 100; j++ {
		if q.Size() != 0 {
			t.Fatal("expected no elements")
		} else if _, err := q.Pop(); err == nil {
			t.Fatal("expected no elements")
		}

		for i := int64(0); i < j; i++ {
			q.Push(i)
		}

		for i := int64(0); i < j; i++ {
			if x, err := q.Pop(); err != nil {
				t.Fatal("expected an element")
			} else if x != i {
				t.Fatalf("expected %d got %d", i, x)
			}
		}
	}

	a := int64(0)
	r := int64(0)
	for j := int64(0); j < 100; j++ {
		for i := int64(0); i < 4; i++ {
			q.Push(a)
			a++
		}

		for i := int64(0); i < 2; i++ {
			if x, err := q.Pop(); err != nil {
				t.Fatal("expected an element")
			} else if x != r {
				t.Fatalf("expected %d got %d", r, x)
			}

			r++
		}
	}

	if q.Size() != 200 {
		t.Fatalf("expected 200 elements have %d", q.Size())
	}
}

func TestTimeQueue(t *testing.T) {
	q := NewTimeQueue(2)
	zerotime := time.Now()
	looptime := zerotime.Add(time.Duration(100))

	for j := zerotime; j.Before(looptime); j = j.Add(time.Duration(1)) {
		if q.Size() != 0 {
			t.Fatal("expected no elements")
		} else if _, err := q.Pop(); err == nil {
			t.Fatal("expected no elements")
		}

		for i := zerotime; i.Before(j); i = i.Add(time.Duration(1)) {
			k := i
			q.Push(&k)
		}

		for i := zerotime; i.Before(j); i = i.Add(time.Duration(1)) {
			if x, err := q.Pop(); err != nil {
				t.Fatal("expected an element")
			} else if *x != i {
				t.Fatalf("expected %s got %s", i, x)
			}
		}
	}

	a := zerotime
	r := zerotime
	for j := 0; j < 100; j++ {
		for i := 0; i < 4; i++ {
			p := a
			q.Push(&p)
			a = a.Add(time.Duration(1))
		}

		for i := 0; i < 2; i++ {
			if x, err := q.Pop(); err != nil {
				t.Fatal("expected an element")
			} else if *x != r {
				t.Fatalf("expected %d got %d", r, x)
			}

			r = r.Add(time.Duration(1))
		}
	}

	if q.Size() != 200 {
		t.Fatalf("expected 200 elements have %d", q.Size())
	}
}

func BenchmarkWordAdd(b *testing.B) {
	b.ReportAllocs()
	q := NewWordQueue(2)

	for i := int64(0); i < int64(b.N); i++ {
		q.Push(i)
	}
}

func BenchmarkWordRemove(b *testing.B) {
	b.ReportAllocs()
	q := NewWordQueue(2)

	for i := int64(0); i < int64(b.N); i++ {
		q.Push(i)

		if q.Size() > 10 {
			q.Pop()
		}
	}
}

func BenchmarkTimeAdd(b *testing.B) {
	b.ReportAllocs()
	q := NewTimeQueue(2)

	zerotime := time.Now()
	looptime := zerotime.Add(time.Duration(b.N))
	for j := zerotime; j.Before(looptime); j = j.Add(time.Duration(1)) {
		p := j
		q.Push(&p)
	}
}

func BenchmarkTimeRemove(b *testing.B) {
	b.ReportAllocs()
	q := NewTimeQueue(2)

	zerotime := time.Now()
	looptime := zerotime.Add(time.Duration(b.N))
	for j := zerotime; j.Before(looptime); j = j.Add(time.Duration(1)) {
		p := j
		q.Push(&p)

		if q.Size() > 10 {
			q.Pop()
		}
	}
}
