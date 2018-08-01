package krypton

import (
	"encoding/json"
	"net/url"

	logging "github.com/op/go-logging"
	"github.com/pkg/errors"
)

type Client struct {
	cfg *Config
}

var (
	logger *logging.Logger
)

func log(format string, args ...interface{}) {
	if logger != nil {
		logger.Debugf(format, args...)
	}
}

func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("config must be specified")
	}
	logger = cfg.Logger

	if cfg.ProvisioningAPIEndpointURL == nil {
		u, err := url.Parse("https://g.api.soracom.io/")
		if err != nil {
			return nil, err
		}
		cfg.ProvisioningAPIEndpointURL = u
	}

	return &Client{
		cfg: cfg,
	}, nil
}

func (c *Client) Close() {
	// Do nothing
}

func (c *Client) PerformOperation(operationName string) error {
	op, ok := operations[operationName]
	if !ok {
		return errors.Errorf("unknown operation name: %s", operationName)
	}
	return op.Perform(c)
}

func (c *Client) getValueFromRequestParameterOption(name string) (interface{}, error) {
	if c.cfg.RequestParameters == "" {
		return nil, errors.Errorf("parameter '%s' must be specified in -params option", name)
	}

	var m map[string]interface{}
	err := json.Unmarshal([]byte(c.cfg.RequestParameters), &m)
	if err != nil {
		return nil, errors.Errorf("unable parse -params / -p option")
	}

	v, found := m[name]
	if !found {
		return nil, errors.Errorf("no parameter found with the name %s in request parameters", name)
	}

	return v, nil
}
