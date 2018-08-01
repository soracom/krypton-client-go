package endorse

import (
	"net/url"

	logging "github.com/op/go-logging"
)

type Config struct {
	KeysAPIEndpointURL *url.URL
	SignatureAlgorithm string
	UICCInterfaceType  UICCInterfaceType
	KeyCache           KeyCacheConfig
	Serial             SerialConfig
	Logger             *logging.Logger
}

type KeyCacheConfig struct {
	Disabled bool
	Clear    bool
}

type SerialConfig struct {
	PortName   string
	BaudRate   uint
	DataBits   uint
	StopBits   uint
	ParityMode ParityMode
}

type ParityMode int

const (
	ParityModeNone ParityMode = iota
	ParityModeOdd
	ParityModeEven
)
