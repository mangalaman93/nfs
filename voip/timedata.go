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
	step int
	wl   int

	// vars
	since int
	pval  int64
	pts   *time.Time
}

// all data in ms
func NewTimeData(step int, duration int, bts time.Time) *TimeData {
	length := duration / step * 2

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
	t.ts.Push(&ts)
}

func (t *TimeData) Next() (int64, float32, bool) {
	t.bts.Add(time.Millisecond * time.Duration(t.step))
	t.since += t.step

	for {
		cval, err := t.data.Head()
		cts, err := t.ts.Head()
		if err != nil {
			return 0, 0, false
		}

		// continue if we don't have latest timestamp
		if cts.Before(t.bts) {
			t.pts, _ = t.ts.Pop()
			t.pval, _ = t.data.Pop()
			continue
		}

		// otherwise we have the latest timestamp
		tdiff := cts.Sub(*t.pts) * 10e9
		if t.pval == 0 || tdiff == 0 {
			return cval, 0, true
		} else {
			return cval, float32(cval-t.pval) / float32(tdiff), true
		}

	}
}

func (t *TimeData) AfterD() int {
	if t.since < t.wl {
		return 0
	}

	ret := t.since
	t.since = 0
	return ret
}
