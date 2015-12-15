package voip

import (
	"log"
	"math"
	"time"

	"github.com/influxdb/influxdb/models"
)

const (
	RX_TABLE = iota
	TX_TABLE
	CPU_TABLE
	QUEUE_TABLE
)

type MContainer struct {
	node *Node

	// data
	inflow  *TimeData
	outflow *TimeData
	cpuload *TimeData
	queue   *TimeData

	// control vars
	shares int64
	ref    int64
	alpha  float64

	// algorithm vars
	ploadr  float64
	prxr    float64
	ptxr    float64
	pqueuel int64
	ibytes  int64
	tibytes int64
	csum    float64
}

func NewMContainer(node *Node, step, wl, shares, ref int64, alpha float64) *MContainer {
	curtime := time.Now()

	return &MContainer{
		node:    node,
		inflow:  NewTimeData(step, wl, curtime),
		outflow: NewTimeData(step, wl, curtime),
		cpuload: NewTimeData(step, wl, curtime),
		queue:   NewTimeData(step, wl, curtime),
		shares:  shares,
		ref:     ref,
		alpha:   alpha,
	}
}

func (m *MContainer) AddPoint(table int, point models.Point) {
	fval, ok := point.Fields()["value"].(float64)
	if !ok {
		log.Println("[WARN] unknown data type!")
		return
	}

	val := int64(fval)
	switch table {
	case RX_TABLE:
		m.inflow.AddPoint(point.Time(), val)
	case TX_TABLE:
		m.outflow.AddPoint(point.Time(), val)
	case CPU_TABLE:
		m.cpuload.AddPoint(point.Time(), val)
	case QUEUE_TABLE:
		m.queue.AddPoint(point.Time(), val)
	default:
		panic("NOT REACHABLE")
	}
}

func (m *MContainer) Trigger() int64 {
	flag := false

	for {
		_, rxr, ok1 := m.inflow.Next()
		tx, txr, ok2 := m.outflow.Next()
		_, cpr, ok3 := m.cpuload.Next()
		lqueue, _, ok4 := m.queue.Next()
		cpr /= 10000000
		if !ok1 || !ok2 || !ok3 || !ok4 {
			break
		}

		if m.ibytes == 0 {
			m.ibytes = tx
			m.tibytes = tx
		}

		// we have three points rx, tx, cp synchronized within <step>
		// ninetyp := float64(m.shares) * 90 / 1024
		switch {
		case m.pqueuel > 0 && lqueue <= 0:
			m.csum += float64(tx-m.tibytes) * (m.prxr / (m.prxr + m.ptxr))
			m.tibytes = tx
		case m.pqueuel <= 0 && lqueue > 0:
			m.tibytes = tx
		case m.pqueuel > 0 && lqueue > 0:
		case m.pqueuel <= 0 && lqueue <= 0:
		}

		// 1 interval is over, we look at only inflow for consistency
		duration := float64(m.inflow.AfterD()) / 1000
		if duration > 0 {
			m.csum += float64(tx-m.tibytes) * (m.prxr / (m.prxr - m.ptxr))
			dprime := float64(m.csum) / float64(m.shares) / duration
			if math.Abs(dprime) > 0 && math.Abs(dprime) < 1000000 {
				delta := float64(tx-m.ibytes) / duration
				log.Println("m.sum:", m.csum, "dprime:", dprime, "delta", delta)
				m.shares += int64(m.alpha * (float64(m.ref) - delta) / dprime)
				if m.shares < 0 {
					m.shares = 64
				} else if m.shares > 1024 {
					m.shares = 1024
				}
				flag = true
			}

			m.csum = 0
			m.ibytes = tx
			m.tibytes = tx
		}

		m.ploadr = cpr
		m.prxr = rxr
		m.ptxr = txr
		m.pqueuel = lqueue
	}

	if flag {
		return m.shares
	} else {
		return 0
	}
}
