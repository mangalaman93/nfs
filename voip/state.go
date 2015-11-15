package voip

import (
	"log"

	"github.com/Unknwon/goconfig"
	"github.com/influxdb/influxdb/models"
)

const (
	UBUFSIZE = 100
)

type Container struct {
	id      string
	inflow  *TimeData
	outflow *TimeData
	cpuload *TimeData
}

func NewContainer(id string, s, d int) *Container {
	return &Container{
		id:      id,
		inflow:  NewTimeData(s, d),
		outflow: NewTimeData(s, d),
		cpuload: NewTimeData(s, d),
	}
}

type State struct {
	// control parameters
	nfconts map[string]*Container
	uchan   chan string
	rchan   chan bool
	mger    CManager

	// config parameters
	step_length     int
	period_length   int
	cpu_usage_table string
	rx_table        string
	tx_table        string
}

func NewState(config *goconfig.ConfigFile) (*State, error) {
	step_length, err := config.Int("VOIP.CONTROL", "step_length")
	if err != nil {
		return nil, err
	}

	period_length, err := config.Int("VOIP.CONTROL", "period_length")
	if err != nil {
		return nil, err
	}

	cpu_table, err := config.GetValue("VOIP.CONTROL", "cpu_table")
	if err != nil {
		return nil, err
	}

	rx_table, err := config.GetValue("VOIP.CONTROL", "rx_table")
	if err != nil {
		return nil, err
	}

	tx_table, err := config.GetValue("VOIP.CONTROL", "tx_table")
	if err != nil {
		return nil, err
	}

	var mger CManager
	mtype, err := config.GetValue("VOIP.MANAGER", "type")
	if err != nil {
		return nil, err
	}
	switch mtype {
	case "docker":
		mger, err = NewDockerCManager(config)
	case "stack":
		mger, err = NewStackCManager(config)
	default:
		err = ErrInvalidManagerType
	}
	if err != nil {
		return nil, err
	}

	return &State{
		nfconts: make(map[string]*Container),
		uchan:   make(chan string, UBUFSIZE),
		rchan:   make(chan bool, UBUFSIZE),
		mger:    mger,

		step_length:     step_length,
		period_length:   period_length,
		cpu_usage_table: cpu_table,
		rx_table:        rx_table,
		tx_table:        tx_table,
	}, nil
}

func (s *State) Destroy() {
	close(s.uchan)
	close(s.rchan)
}

func (s *State) Trigger() {
	// TODO
}

func (s *State) Update(points models.Points) {
	select {
	case nf := <-s.uchan:
		s.nfconts[nf] = NewContainer(nf, s.step_length, s.period_length)
	default:
	}

	if len(s.nfconts) == 0 {
		return
	}

	for _, point := range points {
		switch point.Name() {
		case s.cpu_usage_table:
		case s.rx_table:
		case s.tx_table:
			s.addPoint(point)
		}
	}

	s.Trigger()
}

func (s *State) addPoint(point models.Point) {
	cont, ok := s.nfconts[point.Tags()["container_name"]]
	if !ok {
		return
	}

	val, ok := point.Fields()["value"].(int64)
	if !ok {
		log.Println("[WARN] unknown data type!")
		return
	}

	switch point.Name() {
	case s.cpu_usage_table:
		cont.cpuload.AddPoint(point.Time(), val)
	case s.rx_table:
		cont.inflow.AddPoint(point.Time(), val)
	case s.tx_table:
		cont.outflow.AddPoint(point.Time(), val)
	}
}
