package endorse

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"
)

type Comm struct {
	cfg  *SerialConfig
	port io.ReadWriteCloser
}

func newComm(cfg *SerialConfig) (*Comm, error) {
	options := serial.OpenOptions{
		PortName:              cfg.PortName,
		BaudRate:              cfg.BaudRate,
		DataBits:              cfg.DataBits,
		StopBits:              cfg.StopBits,
		ParityMode:            convertParityMode(cfg.ParityMode),
		InterCharacterTimeout: 100,
	}

	log("opening comm port: %s", cfg.PortName)
	port, err := serial.Open(options)
	if err != nil {
		return nil, err
	}

	c := &Comm{
		cfg:  cfg,
		port: port,
	}

	log("initializing comm port: %s", cfg.PortName)
	_, err = c.sendATCommandSync("ATE0V1")
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Comm) sendATCommandSync(cmd string) (string, error) {
	log("sending %s", cmd)
	fmt.Fprintf(c.port, cmd+"\r\n")
	r := bufio.NewReader(c.port)
	s := ""
	for {
		ln, _, err := r.ReadLine()
		if err != nil && err != io.EOF {
			return "", err
		}

		ls := string(ln)

		if err == io.EOF || strings.HasSuffix(ls, "OK") {
			return s, nil
		}

		s = s + ls + "\n"
		if strings.HasSuffix(ls, "ERROR") || strings.Index(ls, "+CME ERROR") > 0 {
			return "", errors.New(s)
		}
	}
}

func convertParityMode(pm ParityMode) serial.ParityMode {
	switch pm {
	case ParityModeOdd:
		return serial.PARITY_ODD
	case ParityModeEven:
		return serial.PARITY_EVEN
	default:
		return serial.PARITY_NONE
	}
}

func (c *Comm) Close() {
	c.port.Close()
}

func (c *Comm) String() string {
	return c.cfg.PortName
}

func (c *Comm) ReadIMSI() (string, error) {
	result, err := c.sendATCommandSync("AT+CIMI")
	if err != nil {
		return "", err
	}
	log("response from SIM:\n```\n%s\n```\n", result)

	s := strings.TrimSpace(result)
	return s, nil
}

func (c *Comm) Authenticate(rand []byte, autn []byte) (*UICCAuthResult, error) {
	b := []byte{0x00, 0x88, 0x00, 0x81, byte(len(rand) + len(autn) + 2)}
	b = append(b, byte(len(rand)))
	b = append(b, rand...)
	b = append(b, byte(len(autn)))
	b = append(b, autn...)
	//b = append(b, 0x00)
	cmd := fmt.Sprintf("AT+CSIM=%d,\"%s\"", len(b)*2, hex.EncodeToString(b))
	result, err := c.sendATCommandSync(cmd)
	if err != nil {
		return nil, err
	}
	log("response from SIM:\n```\n%s\n```\n", result)

	responseBytes, err := parseCSIMResponse(result)
	if len(responseBytes) != 2 {
		return nil, errors.New("unexpected +CSIM response")
	}
	if responseBytes[0] != 0x61 {
		return nil, errors.New("unexpected +CSIM response")
	}

	cmd = fmt.Sprintf("AT+CSIM=10,\"00C00000%2X\"", responseBytes[1])
	result, err = c.sendATCommandSync(cmd)
	if err != nil {
		return nil, err
	}
	log("response from SIM:\n```\n%s\n```\n", result)

	responseBytes, err = parseCSIMResponse(result)
	if responseBytes[len(responseBytes)-2] != 0x90 || responseBytes[len(responseBytes)-1] != 0x00 {
		return nil, errors.New("unexpected +CSIM response")
	}

	return parseUICCAuthResult(responseBytes)
}

var regexpCSIMResponse = regexp.MustCompile("[0-9]+,\"([0-9a-fA-F]*)\"")

func parseCSIMResponse(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "+CSIM:")
	s = strings.TrimSpace(s)
	sm := regexpCSIMResponse.FindStringSubmatch(s)
	if len(sm) < 2 {
		return nil, errors.New("unexpected +CSIM response")
	}

	return hex.DecodeString(sm[1])
}
