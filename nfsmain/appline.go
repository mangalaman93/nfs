package nfsmain

import (
	"github.com/influxdb/influxdb/models"
)

type AppLine interface {
	Start() error
	Stop()
	GetDB() string

	// should be able to handle concurrent calls
	Update(points models.Points)
}
