package voip

import (
	"testing"
	"time"
)

var (
	zero = time.Unix(0, 0)
)

func testCase(t *testing.T, d *TimeData, ts [][]time.Time, val [][]int64,
	iduration []int64, isum []int64, icount []int64) {
	index := 0
	sum := int64(0)
	count := int64(0)

	if len(ts) != len(val) || len(isum) != len(icount) {
		t.Fatal("incorrect test case")
	}

	for i, _ := range ts {
		if len(ts[i]) != len(val[i]) {
			t.Fatal("incorrect test case")
		}

		for j, _ := range ts[i] {
			d.AddPoint(ts[i][j], val[i][j])
		}

		for {
			v, _, ok := d.Next()
			if !ok {
				break
			}

			sum += v
			count++

			duration := d.AfterD()
			switch {
			case duration > 0:
				if iduration[index] != duration {
					t.Errorf("duration is not correct, got %d, expected %d", duration, iduration[index])
				}
				if isum[index] != sum {
					t.Errorf("sum is not correct, got %d, expected %d", sum, isum[index])
				}
				if icount[index] != count {
					t.Errorf("count is not correct, got %d, expected %d", count, icount[index])
				}

				index++
				sum = 0
				count = 0
			}
		}
	}

	if index != len(isum) {
		t.Errorf("different number of intervals, got %d expected %d", index, len(isum))
	}
}

/* +------+----+----+----+----+----+-----+-----+-----+-----+
   | time | 40 | 60 | 80 | 90 | 99 | 100 | 101 | 140 | 240 |
   +------+----+----+----+----+----+-----+-----+-----+-----+
   | val  | 1  | 2  | 3  | 4  | 5  | 6   | 7   | 8   | 9   |
   +------+----+----+----+----+----+-----+-----+-----+-----+ */
func TestCaseOne(t *testing.T) {
	testCase(t,
		NewTimeData(10, 100, zero),
		getTimeSlice([][]int64{[]int64{40, 60, 80, 90, 99, 100, 101, 140, 240}}),
		[][]int64{[]int64{1, 2, 3, 4, 5, 6, 7, 8, 9}},
		[]int64{100, 140},
		[]int64{4*1 + 2*2 + 2*3 + 1*4 + +1*6, 8*4 + 9*6},
		[]int64{10, 10})
}

func TestCaseTwo(t *testing.T) {
	testCase(t,
		NewTimeData(10, 100, zero),
		getTimeSlice([][]int64{[]int64{40, 60, 80}, []int64{90, 99, 100, 101, 140, 240}}),
		[][]int64{[]int64{1, 2, 3}, []int64{4, 5, 6, 7, 8, 9}},
		[]int64{100, 140},
		[]int64{4*1 + 2*2 + 2*3 + 1*4 + 1*6, 8*4 + 9*6},
		[]int64{10, 10})
}

func TestCaseThree(t *testing.T) {
	testCase(t,
		NewTimeData(10, 100, zero),
		getTimeSlice([][]int64{[]int64{40, 60, 80}, []int64{90, 99, 100}, []int64{101, 140, 240}}),
		[][]int64{[]int64{1, 2, 3}, []int64{4, 5, 6}, []int64{7, 8, 9}},
		[]int64{100, 140},
		[]int64{4*1 + 2*2 + 2*3 + 1*4 + 1*6, 8*4 + 9*6},
		[]int64{10, 10})
}

/* +------+----+----+----+----+----+----+----+----+----+-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+
   | time | 10 | 20 | 30 | 40 | 50 | 60 | 70 | 80 | 90 | 100 | 120 | 130 | 140 | 150 | 160 | 170 | 180 | 190 | 200 |
   +------+----+----+----+----+----+----+----+----+----+-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+
   | val  | 1  | 2  | 3  | 4  | 5  | 6  | 7  | 8  | 9  | 10  | 11  | 12  | 13  | 14  | 15  | 16  | 17  | 18  | 19  |
   +------+----+----+----+----+----+----+----+----+----+-----+-----+-----+-----+-----+-----+-----+-----+-----+-----+ */
func TestCaseFour(t *testing.T) {
	testCase(t,
		NewTimeData(10, 100, zero),
		getTimeSlice([][]int64{[]int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 120, 130, 140, 150, 160, 170, 180, 190, 200}}),
		[][]int64{[]int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}},
		[]int64{100, 100},
		[]int64{55, 146},
		[]int64{10, 10})
}

func getTimeSlice(v2d [][]int64) [][]time.Time {
	ts := make([][]time.Time, len(v2d))

	for i, v1d := range v2d {
		ts[i] = make([]time.Time, len(v1d))
		for j, v := range v1d {
			ts[i][j] = zero.Add(time.Duration(v) * time.Millisecond)
		}
	}

	return ts
}
