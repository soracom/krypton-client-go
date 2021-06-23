package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"github.com/soracom/endorse-client-go/endorse"
	"github.com/soracom/krypton-client-go/krypton"
)

var log = logging.MustGetLogger("krypton-cli")
var formatNormal = logging.MustStringFormatter(
	`%{color} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)
var formatDebug = logging.MustStringFormatter(
	`%{color}%{time:15:04:05.000} %{shortfile} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

type runMode int

const (
	runModeNormal runMode = iota
	runModeListCOMPorts
	runModeDeviceInfo
	runModePerformSpecifiedOperation
	runModeDoNothing
	runModeUnknown
)

type appConfig struct {
	Operation string
	Debug     bool
}

func main() {
	rand.Seed(time.Now().UnixNano())

	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	rm, appCfg, endorseCfg, kryptonCfg, err := parseFlags()
	if err != nil {
		return err
	}
	if rm == runModeDoNothing {
		return nil
	}

	setupLogger(appCfg)

	ec, err := endorse.NewClient(endorseCfg)
	if err != nil {
		return err
	}
	defer ec.Close()

	kryptonCfg.EndorseClient = ec

	kc, err := krypton.NewClient(kryptonCfg)
	if err != nil {
		return err
	}
	defer kc.Close()

	switch rm {
	case runModeListCOMPorts:
		return listCOMPorts(ec)
	case runModeDeviceInfo:
		return deviceInfo(ec)
	case runModePerformSpecifiedOperation:
		return performSpecifiedOperation(appCfg, kc)
	default:
		return errors.New("unknown run mode")
	}
}

func parseFlags() (runMode, *appConfig, *endorse.Config, *krypton.Config, error) {
	var (
		operation                  string
		provisioningAPIEndpointURL string
		requestParameters          string
		keysAPIEndpointURL         string
		signatureAlgorithm         string

		uiccInterfaceType string
		portName          string
		baudRate          uint
		dataBits          uint
		stopBits          uint
		parityMode        uint

		listCOMPorts bool
		deviceInfo   bool

		disableKeyCache bool
		clearKeyCache   bool

		help    bool
		version bool
		debug   bool
	)
	operationHelpText := krypton.GenerateOperationsHelpText()
	flag.StringVar(&operation, "operation", "", operationHelpText)
	flag.StringVar(&provisioningAPIEndpointURL, "provisioning-api-endpoint-url", "", "Use the specified URL as a Provisioning API endpoint. (default: https://g.api.soracom.io/)")
	flag.StringVar(&requestParameters, "params", "", "Pass additional JSON parameters to the service request")
	flag.StringVar(&requestParameters, "p", "", "Pass additional JSON parameters to the service request")
	flag.StringVar(&keysAPIEndpointURL, "keys-api-endpoint-url", "", "Use the specified URL as a Keys API endpoint")
	flag.StringVar(&signatureAlgorithm, "signature-algorithm", "SHA-256", "Algorithm for generating signature. (default is SHA-256)")

	flag.StringVar(&uiccInterfaceType, "interface", "autoDetect", "UICC Interface to use. Valid values are iso7816, comm, mmcli or autoDetect")
	flag.StringVar(&portName, "port-name", "", "Port name of communication device (e.g. -c COM1 or -c /dev/tty1)")
	flag.UintVar(&baudRate, "baud-rate", 57600, "Baud rate for communication device (e.g. -b 57600)")
	flag.UintVar(&dataBits, "data-bits", 8, "Data bits for communication device (e.g. -s 8)")
	flag.UintVar(&stopBits, "stop-bits", 1, "Stop bits for communication device (e.g. -s 1)")
	flag.UintVar(&parityMode, "parity-mode", 0, "Parity mode for communication device. 0: None, 1: Odd, 2: Even")

	flag.BoolVar(&listCOMPorts, "list-com-ports", false, "List all available communication devices and exit")
	flag.BoolVar(&deviceInfo, "device-info", false, "Query the communication device and print the information")

	flag.BoolVar(&disableKeyCache, "disable-key-cache", false, "Do not store authentication result to the key cache")
	flag.BoolVar(&clearKeyCache, "clear-key-cache", false, "Remove all items in the key cache")

	flag.BoolVar(&help, "help", false, "Display this help message and exit")
	flag.BoolVar(&help, "h", false, "Display this help message and exit")
	flag.BoolVar(&version, "version", false, "Show version number")
	flag.BoolVar(&debug, "debug", false, "Show verbose debug messages")
	flag.Parse()

	if help {
		flag.Usage()
		return runModeDoNothing, nil, nil, nil, nil
	}
	if version {
		showVersion()
		return runModeDoNothing, nil, nil, nil, nil
	}

	appCfg := &appConfig{
		Operation: operation,
		Debug:     debug,
	}

	setupLogger(appCfg)

	var err error
	var kaeu *url.URL
	if keysAPIEndpointURL != "" {
		kaeu, err = url.Parse(keysAPIEndpointURL)
		if err != nil {
			return runModeUnknown, nil, nil, nil, err
		}
	}

	uit, err := endorse.ParseUICCInterfaceType(uiccInterfaceType)
	if err != nil {
		return runModeUnknown, nil, nil, nil, err
	}

	serial := endorse.SerialConfig{
		PortName:   portName,
		BaudRate:   baudRate,
		DataBits:   dataBits,
		StopBits:   stopBits,
		ParityMode: endorse.ParityMode(parityMode),
	}

	eCfg := &endorse.Config{
		KeysAPIEndpointURL: kaeu,
		SignatureAlgorithm: signatureAlgorithm,
		UICCInterfaceType:  *uit,
		Serial:             serial,
		Logger:             log,
	}

	var paeu *url.URL
	if provisioningAPIEndpointURL != "" {
		paeu, err = url.Parse(provisioningAPIEndpointURL)
		if err != nil {
			return runModeUnknown, nil, nil, nil, err
		}
	}

	kCfg := &krypton.Config{
		ProvisioningAPIEndpointURL: paeu,
		RequestParameters:          requestParameters,
		Logger:                     log,
	}

	if listCOMPorts {
		eCfg.UICCInterfaceType = endorse.UICCInterfaceTypeNone
		return runModeListCOMPorts, appCfg, eCfg, kCfg, nil
	}

	if deviceInfo {
		eCfg.UICCInterfaceType = endorse.UICCInterfaceTypeComm
		if portName == "" {
			return runModeUnknown, nil, nil, nil, errors.New("-port-name must be specified with -device-info")
		}
		return runModeDeviceInfo, appCfg, eCfg, kCfg, nil
	}

	if operation == "" {
		return runModeUnknown, nil, nil, nil, errors.New("operation must be specified")
	}

	return runModePerformSpecifiedOperation, appCfg, eCfg, kCfg, nil
}

func setupLogger(appCfg *appConfig) {
	be := logging.NewLogBackend(os.Stderr, "", 0)
	format := formatNormal
	if appCfg.Debug {
		format = formatDebug
	}
	bf := logging.NewBackendFormatter(be, format)
	ml := logging.AddModuleLevel(bf)
	level := logging.ERROR
	if appCfg.Debug {
		level = logging.DEBUG
	}
	ml.SetLevel(level, "")
	logging.SetBackend(ml)
}

func listCOMPorts(ec *endorse.Client) error {
	ports, err := ec.ListCOMPorts()
	if err != nil {
		return err
	}

	fmt.Println(strings.Join(ports, "\n"))
	return nil
}

func deviceInfo(ec *endorse.Client) error {
	di, err := ec.GetDeviceInfo()
	if err != nil {
		return err
	}

	fmt.Println(di)
	return nil
}

func performSpecifiedOperation(appCfg *appConfig, kc *krypton.Client) error {
	return kc.PerformOperation(appCfg.Operation)
}

func showVersion() error {
	fmt.Println(Version)
	return nil
}
