package voip

import (
	"time"

	"github.com/mangalaman93/nfs/pkg/queue"
)

type TimeData struct {
	data *queue.WordQueue
	ts   *queue.TimeQueue

	// parameters
	bts  time.Time
	step int64
	wl   int64

	// vars
	since int64
	pval  int64
	pts   time.Time
}

// all data in ms
func NewTimeData(step int64, duration int64, bts time.Time) *TimeData {
	length := int(duration / step * 2)

	return &TimeData{
		data: queue.NewWordQueue(length),
		ts:   queue.NewTimeQueue(length),
		bts:  bts,
		step: step,
		wl:   duration,
	}
}

func (t *TimeData) AddPoint(ts time.Time, val int64) {
	t.data.Push(val)
	t.ts.Push(ts)
}

func (t *TimeData) Next() (int64, float64, bool) {
	flag := true
	temp_bts := t.bts.Add(time.Millisecond * time.Duration(t.step))

	for {
		cval, err := t.data.Head()
		cts, err := t.ts.Head()
		if err != nil {
			return 0, 0, false
		}

		// continue if we don't have latest timestamp
		if cts.Before(temp_bts) {
			t.pts, _ = t.ts.Pop()
			t.pval, _ = t.data.Pop()
			continue
		}

		if flag {
			flag = false
			t.bts = t.bts.Add(time.Millisecond * time.Duration(t.step))
			t.since += t.step
		}

		// otherwise we have the latest timestamp
		tdiff := float64(cts.Sub(t.pts)) / 1e9
		if t.pval == 0 || tdiff == 0 {
			return cval, 0, true
		} else {
			return cval, float64(cval-t.pval) / tdiff, true
		}
	}
}

func (t *TimeData) AfterD() int64 {
	if t.since < t.wl {
		return 0
	}

	cts, err := t.ts.Head()
	if err != nil {
		panic("NOT REACHABLE")
	}

	ret := t.since + int64(cts.Sub(t.bts))/1e6
	t.since = 0
	return ret
}
