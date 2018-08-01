package krypton

import (
	"net/url"

	"github.com/op/go-logging"
	"github.com/soracom/endorse-client-go/endorse"
)

type Config struct {
	ProvisioningAPIEndpointURL *url.URL
	RequestParameters          string
	EndorseClient              *endorse.Client
	Logger                     *logging.Logger
}
