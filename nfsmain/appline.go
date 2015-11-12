package nfsmain

import (
	"github.com/influxdb/influxdb/models"
)

type AppLine interface {
	Start()
	Stop()
	GetDB() string
	Update(points models.Points)
}
