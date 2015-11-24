package voip

import (
	"errors"
	"sync"

	"github.com/Unknwon/goconfig"
	"github.com/influxdb/influxdb/models"
)

var (
	ErrUnknownReq     = errors.New("Unknown request")
	ErrUnknownManager = errors.New("Inavalid container manager type")
)

// TODO: more parallelization here
type VoipHandler struct {
	sync.Mutex

	// control parameters
	mnodes map[string]*MContainer
	anodes map[string]*Node
	cmgr   CManager

	// config parameters
	step_length   int64
	period_length int64
	reference     int64
	cpu_table     string
	rx_table      string
	tx_table      string
}

func NewVoipHandler(config *goconfig.ConfigFile) (*VoipHandler, error) {
	step_length, err := config.Int64("VOIP.CONTROL", "step_length")
	if err != nil {
		return nil, err
	}
	period_length, err := config.Int64("VOIP.CONTROL", "period_length")
	if err != nil {
		return nil, err
	}
	reference, err := config.Int64("VOIP.CONTROL", "reference")
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

	var cmgr CManager
	mtype, err := config.GetValue("VOIP.MANAGER", "type")
	if err != nil {
		return nil, err
	}
	switch mtype {
	case "docker":
		cmgr, err = NewDockerCManager(config)
	default:
		err = ErrUnknownManager
	}
	if err != nil {
		return nil, err
	}

	return &VoipHandler{
		mnodes:        make(map[string]*MContainer),
		anodes:        make(map[string]*Node),
		cmgr:          cmgr,
		step_length:   step_length,
		period_length: period_length,
		reference:     reference,
		cpu_table:     cpu_table,
		rx_table:      rx_table,
		tx_table:      tx_table,
	}, nil
}

func (vh *VoipHandler) Start() error {
	return vh.cmgr.Setup()
}

func (vh *VoipHandler) Stop() {
	vh.Lock()
	defer vh.Unlock()

	for _, mcont := range vh.mnodes {
		vh.cmgr.StopCont(mcont.node)
	}
	for _, node := range vh.anodes {
		vh.cmgr.StopCont(node)
	}

	vh.cmgr.Destroy()
}

func (vh *VoipHandler) HandleRequest(req *Request) *Response {
	vh.Lock()
	defer vh.Unlock()

	switch req.Code {
	case ReqStartServer:
		return vh.addServer(req)
	case ReqStartSnort:
		return vh.addSnort(req)
	case ReqStartClient:
		return vh.addClient(req)
	case ReqStopCont:
		return vh.stopCont(req)
	case ReqRouteCont:
		return vh.route(req)
	default:
		return &Response{Err: ErrUnknownReq.Error()}
	}
}

func (vh *VoipHandler) UpdatePoints(points models.Points) {
	vh.Lock()
	defer vh.Unlock()

	if len(vh.mnodes) == 0 {
		return
	}

	// update points
	for _, point := range points {
		cont, ok := vh.mnodes[point.Tags()["container_name"]]
		if !ok {
			continue
		}

		switch point.Name() {
		case vh.cpu_table:
			cont.AddPoint(CPU_TABLE, point)
		case vh.rx_table:
			cont.AddPoint(RX_TABLE, point)
		case vh.tx_table:
			cont.AddPoint(TX_TABLE, point)
		default:
		}
	}

	// run the algorithm
	for _, mcont := range vh.mnodes {
		shares := mcont.Trigger()
		if shares != 0 {
			vh.cmgr.SetShares(mcont.node, shares)
		}
	}
}
