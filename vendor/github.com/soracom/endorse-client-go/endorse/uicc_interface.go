package endorse

import (
	"github.com/ebfe/scard"
	"github.com/pkg/errors"
)

type UICCInterface interface {
	ReadIMSI() (string, error)
	Authenticate([]byte, []byte) (*UICCAuthResult, error)
	Close()
	String() string
}

func NewUICCInterface(cfg *Config) (UICCInterface, error) {
	switch cfg.UICCInterfaceType {
	case UICCInterfaceTypeAutoDetect:
		return autoDetectUICCInterface(cfg)
	case UICCInterfaceTypeISO7816:
		return newISO7816(0)
	case UICCInterfaceTypeComm:
		return newComm(&cfg.Serial)
	case UICCInterfaceTypeNone:
		return nil, nil
	default:
		return nil, errors.Errorf("unknown UICC interface type: %d", cfg.UICCInterfaceType)
	}
}

func autoDetectUICCInterface(cfg *Config) (UICCInterface, error) {
	detectedCh := make(chan UICCInterface)

	nCommPorts, err := tryCommPorts(cfg, detectedCh)
	if err != nil {
		log("error occurred while trying COM ports: %+v", err)
	}

	nSmartCards, err := trySmartCardReaders(cfg, detectedCh)
	if err != nil {
		log("error occurred while trying smart card readers: %+v", err)
	}

	n := nCommPorts + nSmartCards

	var ui UICCInterface

	for i := 0; i < n; i++ {
		ui = <-detectedCh
		if ui != nil {
			log("found the first working port: %s", ui.String())
			go func() { // read and throw remainings
				for j := 0; j < n-i-1; j++ {
					ui := <-detectedCh
					if ui != nil {
						log("closing: %s", ui.String())
						ui.Close()
					}
				}
			}()
			return ui, nil
		}
	}

	return nil, errors.New("no UICC interface is found")
}

func tryCommPorts(cfg *Config, detectedCh chan UICCInterface) (int, error) {
	portNames, err := listCOMPorts()
	if err != nil {
		return 0, err
	}

	for _, pn := range portNames {
		go tryCommPort(cfg, pn, detectedCh)
	}

	return len(portNames), nil
}

func tryCommPort(cfg *Config, portName string, detectedCh chan UICCInterface) {
	log("trying comm port: %s\n", portName)
	port, err := newComm(&SerialConfig{
		PortName:   portName,
		BaudRate:   cfg.Serial.BaudRate,
		DataBits:   cfg.Serial.DataBits,
		StopBits:   cfg.Serial.StopBits,
		ParityMode: cfg.Serial.ParityMode,
	})
	if err != nil {
		log("unable to open port: %s\n", portName)
		detectedCh <- nil
		return
	}

	log("reading IMSI: %s\n", portName)
	imsi, err := port.ReadIMSI()
	if err != nil {
		log("unable to read IMSI on port: %s\n", portName)
		port.Close()
		detectedCh <- nil
		return
	}

	if imsi == "" {
		log("unable to read IMSI on port: %s\n", portName)
		port.Close()
		detectedCh <- nil
		return
	}

	log("found working port: %s\n", portName)
	detectedCh <- port
}

func trySmartCardReaders(cfg *Config, detectedCh chan UICCInterface) (int, error) {
	ctx, err := scard.EstablishContext()
	if err != nil {
		return 0, err
	}
	defer ctx.Release()

	// List available readers
	readers, err := ctx.ListReaders()
	if err != nil {
		return 0, err
	}

	for i, reader := range readers {
		go trySmartCardReader(cfg, i, reader, detectedCh)
	}

	return len(readers), nil
}

func trySmartCardReader(cfg *Config, index int, readerName string, detectedCh chan UICCInterface) {
	log("trying smart card reader: %s\n", readerName)
	iso7816, err := newISO7816(index)
	if err != nil {
		log("error while creating iso7816 interface %s: %+v", readerName, err)
		detectedCh <- nil
		return
	}

	log("reading IMSI: %s\n", readerName)
	imsi, err := iso7816.ReadIMSI()
	if err != nil {
		log("error while reading imsi on iso7816 interface %s: %+v", readerName, err)
		detectedCh <- nil
		iso7816.Close()
		return
	}

	if imsi == "" {
		log("unable to read imsi on iso7816 interface %s: %+v", readerName, err)
		detectedCh <- nil
		iso7816.Close()
		return
	}

	log("found working smart card: %s\n", readerName)
	detectedCh <- iso7816
}
