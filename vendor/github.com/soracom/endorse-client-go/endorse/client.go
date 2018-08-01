package endorse

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	logging "github.com/op/go-logging"
	"github.com/pkg/errors"
)

type Client struct {
	cfg      *Config
	keyCache keyCache
	ui       UICCInterface
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

	if cfg.KeysAPIEndpointURL == nil {
		u, err := url.Parse("https://g.api.soracom.io/v1/keys")
		if err != nil {
			return nil, err
		}
		cfg.KeysAPIEndpointURL = u
	}

	kc := newKeyCache(cfg)

	ui, err := NewUICCInterface(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{
		cfg:      cfg,
		keyCache: kc,
		ui:       ui,
	}, nil
}

func (c *Client) Close() {
	if c.ui != nil {
		c.ui.Close()
	}
}

func (c *Client) DoAuthentication() (*AuthenticationResult, error) {
	if c.ui == nil {
		return nil, errors.New("unable to open UICC interface")
	}

	imsi, err := c.ui.ReadIMSI()
	if err != nil {
		return nil, err
	}

	var keyID string
	ar, err := c.keyCache.findAuthResult(imsi)
	if err != nil {
		return nil, err
	}
	if ar != nil {
		return ar, nil
	}

	log("start AKA (request 'challenge')")
	chal, err := c.startAKA(imsi, nil, nil)
	if err != nil {
		return nil, err
	}
	keyID = chal.KeyID

	log("authenticate using sim")
	uar, err := c.ui.Authenticate(chal.RAND, chal.AUTN)
	if err != nil {
		return nil, err
	}

	switch uar.Status {
	case AuthStatusSuccess:
		log("finish AKA (send 'response' for the 'challenge')")
		err := c.finishAKA(keyID, uar.RES)
		if err != nil {
			return nil, errors.New("unable to verify master key")
		}

	case AuthStatusSynchronisationFailure:
		log("restart AKA (resync)")
		chal, err = c.startAKA(imsi, chal.RAND, uar.AUTS)
		if err != nil {
			return nil, err
		}
		keyID = chal.KeyID

		log("authenticate using sim")
		uar, err = c.ui.Authenticate(chal.RAND, chal.AUTN)
		if err != nil {
			return nil, err
		}

		log("verifying master key")
		err = c.finishAKA(keyID, uar.RES)
		if err != nil {
			return nil, err
		}
	}

	ar = &AuthenticationResult{
		KeyID: keyID,
		IMSI:  imsi,
		CK:    uar.CK,
	}

	log("saving master key to key cache")
	err = c.keyCache.saveAuthResult(imsi, ar)
	if err != nil {
		log("error occurred while saving master key to key cache: %+v", err)
	}

	return ar, nil
}

func (c *Client) startAKA(imsi string, rand, auts []byte) (*challenge, error) {
	d := struct {
		IMSI string `json:"imsi"`
		RAND []byte `json:"rand,omitempty"`
		AUTS []byte `json:"auts,omitempty"`
	}{
		IMSI: imsi,
		RAND: rand,
		AUTS: auts,
	}

	reqBodyBytes, err := json.Marshal(&d)
	if err != nil {
		return nil, err
	}

	url := c.cfg.KeysAPIEndpointURL
	log("request url == %s, body == %s\n", url.String(), string(reqBodyBytes))
	resp, err := http.Post(url.String(), "application/json", bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	log("status == %d, resp == %s\n", resp.StatusCode, string(respBodyBytes))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return nil, errors.Errorf("key agreement url responded with error: %s", resp.Status)
	}

	var chal challenge
	err = json.Unmarshal(respBodyBytes, &chal)
	if err != nil {
		return nil, err
	}
	log("challenge == %+v\n", chal)
	return &chal, nil
}

func (c *Client) finishAKA(keyID string, res []byte) error {
	reqBody := struct {
		RES []byte `json:"res"`
	}{
		RES: res,
	}

	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/%s/verify", c.cfg.KeysAPIEndpointURL.String(), keyID)
	log("url: %s, sending body == %+v\n", url, reqBody)
	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBodyBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("unsuccessful response from key agreement server")
	}

	return nil
}

func (c *Client) PostWithSignature(url *url.URL, ck []byte, reqBody interface{}) (*http.Response, error) {
	log("posting request to a service: %+v", reqBody)
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	log("posting JSON to a service: %s", string(reqBodyBytes))

	timestampMillis := time.Now().UnixNano() / 1000 / 1000
	timestampMillisStr := fmt.Sprintf("%d", timestampMillis)

	sig, err := c.calculateSignature(reqBodyBytes, timestampMillisStr, ck)
	if err != nil {
		return nil, err
	}
	log("calculated signature: %s", hex.EncodeToString(sig))

	req, err := http.NewRequest("POST", url.String(), bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-soracom-timestamp", timestampMillisStr)
	req.Header.Set("x-soracom-digest-algorithm", c.cfg.SignatureAlgorithm)
	req.Header.Set("x-soracom-signature", base64.StdEncoding.EncodeToString(sig))

	log("sending request: %+v", req)
	log("sending body: %s", string(reqBodyBytes))

	return http.DefaultClient.Do(req)
}

func (c *Client) calculateSignature(body []byte, timestampMillisStr string, ck []byte) ([]byte, error) {
	alg := c.cfg.SignatureAlgorithm
	h := getHashAlgorithmByName(alg)
	if h == nil {
		return nil, errors.Errorf("unknown hash algorithm: %s", alg)
	}
	h.Write(body)
	h.Write([]byte(timestampMillisStr))
	h.Write(ck)
	b := h.Sum(nil)
	return b, nil
}

func (c *Client) ListCOMPorts() ([]string, error) {
	return listCOMPorts()
}

func (c *Client) GetDeviceInfo() (string, error) {
	comm, ok := c.ui.(*Comm)
	if !ok {
		return "", errors.New("get device info works only with comm ports")
	}
	mi, err := comm.sendATCommandSync("AT+CGMI")
	if err != nil {
		return "", err
	}
	mm, err := comm.sendATCommandSync("AT+CGMM")
	if err != nil {
		return "", err
	}
	mr, err := comm.sendATCommandSync("AT+CGMR")
	if err != nil {
		return "", err
	}
	sn, err := comm.sendATCommandSync("AT+CGSN")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`Manufacturer: %s
Model: %s
Revision: %s
S/N: %s
`, strings.TrimSpace(mi), strings.TrimSpace(mm), strings.TrimSpace(mr), strings.TrimSpace(sn)), nil
}

func getHashAlgorithmByName(algo string) hash.Hash {
	switch strings.ToLower(algo) {
	case "sha-256":
		return sha256.New()
	default:
		return nil
	}
}
