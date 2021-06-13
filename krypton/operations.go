package krypton

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

const (
	inventoryApplicationKeyLength = 16
)

var operations map[string]Operation

func init() {
	operations = make(map[string]Operation)

	ops := []Operation{
		&OperationBootstrapArc{},
		&OperationBootstrapAWSIoTThing{},
		&OperationBootstrapInventoryDevice{},
		&OperationGenerateAmazonCognitoOpenIDToken{},
		&OperationGenerateAmazonCognitoSessionCredentials{},
		&OperationGetSubscriberMetadata{},
		&OperationGetUserdata{},
	}
	for _, o := range ops {
		operations[o.GetName()] = o
	}
}

type Operation interface {
	GetName() string
	GetHelpText() string
	Perform(*Client) error
}

type OperationBootstrapArc struct {
}

func (o *OperationBootstrapArc) GetName() string {
	return "bootstrapArc"
}

func (o *OperationBootstrapArc) GetHelpText() string {
	return "perform bootstrap a SORACOM Arc virtual SIM"
}

func (o *OperationBootstrapArc) Perform(kc *Client) error {
	return simpleOperation(kc, "/v1/provisioning/soracom/arc/bootstrap")
}

type OperationBootstrapAWSIoTThing struct {
}

func (o *OperationBootstrapAWSIoTThing) GetName() string {
	return "bootstrapAwsIotThing"
}

func (o *OperationBootstrapAWSIoTThing) GetHelpText() string {
	return "perform bootstrap as an AWS IoT Thing"
}

func (o *OperationBootstrapAWSIoTThing) Perform(kc *Client) error {
	log("performing bootstrapAwsIotThing")

	ec := kc.cfg.EndorseClient

	log("performing authentication")
	ar, err := ec.DoAuthentication()
	if err != nil {
		return err
	}

	u, err := url.Parse(fmt.Sprintf("%s%s", strings.TrimSuffix(kc.cfg.ProvisioningAPIEndpointURL.String(), "/"), "/v1/provisioning/aws/iot/bootstrap"))
	if err != nil {
		return err
	}

	var rp map[string]interface{}
	if kc.cfg.RequestParameters != "" {
		err = json.Unmarshal([]byte(kc.cfg.RequestParameters), &rp)
		if err != nil {
			return err
		}
	}

	reqBody := struct {
		KeyID             string                 `json:"keyId"`
		RequestParameters map[string]interface{} `json:"requestParameters,omitempty"`
	}{
		KeyID:             ar.KeyID,
		RequestParameters: rp,
	}

	resp, err := ec.PostWithSignature(u, ar.CK, reqBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	log("received response: %s", string(respBodyBytes))

	fmt.Println(string(respBodyBytes))

	return nil
}

type OperationBootstrapInventoryDevice struct {
}

func (o *OperationBootstrapInventoryDevice) GetName() string {
	return "bootstrapInventoryDevice"
}

func (o *OperationBootstrapInventoryDevice) GetHelpText() string {
	return "perform bootstrap as an Inventory device"
}

func (o *OperationBootstrapInventoryDevice) Perform(kc *Client) error {
	ec := kc.cfg.EndorseClient
	ar, err := ec.DoAuthentication()
	if err != nil {
		return err
	}

	u, err := url.Parse(fmt.Sprintf("%s%s", strings.TrimSuffix(kc.cfg.ProvisioningAPIEndpointURL.String(), "/"), "/v1/provisioning/soracom/inventory/bootstrap"))
	if err != nil {
		return err
	}

	ep, err := kc.getValueFromRequestParameterOption("endpoint")
	if err != nil {
		return err
	}
	endpoint, ok := ep.(string)
	if !ok {
		return errors.New("endpoint must be a string")
	}

	var rp map[string]interface{}
	if kc.cfg.RequestParameters != "" {
		err = json.Unmarshal([]byte(kc.cfg.RequestParameters), &rp)
		if err != nil {
			return err
		}
	}

	reqBody := struct {
		KeyID             string                 `json:"keyId"`
		Endpoint          string                 `json:"endpoint"`
		RequestParameters map[string]interface{} `json:"requestParameters,omitempty"`
	}{
		KeyID:             ar.KeyID,
		Endpoint:          endpoint,
		RequestParameters: rp,
	}

	resp, err := ec.PostWithSignature(u, ar.CK, reqBody)
	if err != nil {
		return err
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	respMap := make(map[string]interface{})
	err = json.Unmarshal(respBodyBytes, &respMap)
	if err != nil {
		return err
	}

	appKey, err := generateApplicationKeyForInventory(respMap, ar.CK)
	if err != nil {
		return err
	}

	respMap["applicationKey"] = appKey
	respMap = filterMap(respMap, []string{"applicationKey", "serverUri", "pskId"})

	mergedRespBytes, err := json.Marshal(respMap)
	if err != nil {
		return err
	}

	fmt.Println(string(mergedRespBytes))

	return nil
}

func generateApplicationKeyForInventory(m map[string]interface{}, ck []byte) (string, error) {
	appKeyRaw, found := m["applicationKey"]
	if found {
		appKeyStr, ok := appKeyRaw.(string)
		if ok {
			return appKeyStr, nil
		}
	}

	nonceRaw, found := m["nonce"]
	if !found {
		return "", errors.New("nonce is not found in the response from the server")
	}
	nonceStr, ok := nonceRaw.(string)
	if !ok {
		return "", errors.New("nonce must be a string")
	}
	nonceBytes, err := base64.StdEncoding.DecodeString(nonceStr)
	if err != nil {
		return "", err
	}

	timestampRaw, found := m["timestamp"]
	if !found {
		return "", errors.New("timestamp is not found in the response from the server")
	}
	timestampStr, ok := timestampRaw.(string)
	if !ok {
		return "", errors.New("timestamp must be a string")
	}

	appKey := calculateInventoryApplicationKey(nonceBytes, timestampStr, ck)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(appKey), nil
}

func filterMap(m map[string]interface{}, list []string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		for _, s := range list {
			if k == s {
				result[k] = v
			}
		}
	}
	return result
}

func calculateInventoryApplicationKey(nonce []byte, timestampMillis string, ck []byte) []byte {
	h := sha256.New()
	h.Write(nonce)
	h.Write([]byte(timestampMillis))
	h.Write(ck)
	b := h.Sum(nil)

	return b[0:inventoryApplicationKeyLength]
}

type OperationGenerateAmazonCognitoOpenIDToken struct {
}

func (o *OperationGenerateAmazonCognitoOpenIDToken) GetName() string {
	return "generateAmazonCognitoOpenIdToken"
}

func (o *OperationGenerateAmazonCognitoOpenIDToken) GetHelpText() string {
	return "generates an Open ID token using Amazon Cognito"
}

func (o *OperationGenerateAmazonCognitoOpenIDToken) Perform(kc *Client) error {
	return simpleOperation(kc, "/v1/provisioning/aws/cognito/open_id_tokens")
}

type OperationGenerateAmazonCognitoSessionCredentials struct {
}

func (o *OperationGenerateAmazonCognitoSessionCredentials) GetName() string {
	return "generateAmazonCognitoSessionCredentials"
}

func (o *OperationGenerateAmazonCognitoSessionCredentials) GetHelpText() string {
	return "generates a temporary session token using Amazon Cognito"
}

func (o *OperationGenerateAmazonCognitoSessionCredentials) Perform(kc *Client) error {
	return simpleOperation(kc, "/v1/provisioning/aws/cognito/credentials")
}

type OperationGetSubscriberMetadata struct {
}

func (o *OperationGetSubscriberMetadata) GetName() string {
	return "getSubscriberMetadata"
}

func (o *OperationGetSubscriberMetadata) GetHelpText() string {
	return "gets subscriber's metadata"
}

func (o *OperationGetSubscriberMetadata) Perform(kc *Client) error {
	return simpleOperation(kc, "/v1/provisioning/soracom/air/subscriber_metadata")
}

type OperationGetUserdata struct {
}

func (o *OperationGetUserdata) GetName() string {
	return "getUserData"
}

func (o *OperationGetUserdata) GetHelpText() string {
	return "gets userdata from group configuration"
}

func (o *OperationGetUserdata) Perform(kc *Client) error {
	return simpleOperation(kc, "/v1/provisioning/soracom/air/userdata")
}

func simpleOperation(kc *Client, path string) error {
	ec := kc.cfg.EndorseClient
	ar, err := ec.DoAuthentication()
	if err != nil {
		return err
	}

	u, err := url.Parse(fmt.Sprintf("%s%s", strings.TrimSuffix(kc.cfg.ProvisioningAPIEndpointURL.String(), "/"), path))
	if err != nil {
		return err
	}

	var rp map[string]interface{}
	if kc.cfg.RequestParameters != "" {
		err = json.Unmarshal([]byte(kc.cfg.RequestParameters), &rp)
		if err != nil {
			return err
		}
	}

	reqBody := struct {
		KeyID             string                 `json:"keyId"`
		RequestParameters map[string]interface{} `json:"requestParameters,omitempty"`
	}{
		KeyID:             ar.KeyID,
		RequestParameters: rp,
	}

	resp, err := ec.PostWithSignature(u, ar.CK, reqBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode > http.StatusBadRequest {
		return errors.Errorf("unsuccessful response: %s", resp.Status)
	}

	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println(string(respBodyBytes))

	return nil
}

func GenerateOperationsHelpText() string {
	names := make([]string, 0, len(operations))
	for name := range operations {
		names = append(names, name)
	}
	sort.Strings(names)

	texts := make([]string, len(operations))
	for i, name := range names {
		texts[i] = operations[name].GetHelpText()
	}

	n := getMaxLength(names)

	formattedLines := []string{}
	for i := range names {
		f := fmt.Sprintf("\t%%-%ds%%s", n+3)
		line := fmt.Sprintf(f, names[i], texts[i])
		formattedLines = append(formattedLines, line)
	}

	s := strings.Join(formattedLines, "\n")

	/*
		contet of `s`` should be like:

		getSubscirberMetadata                     gets subscriber's metadata
		getUserData                               gets 'userdata' from group configuration
		bootstrapAwsIotThings                     perform bootstrap for AWS IoT Things
		bootstrapInventoryDevice                  perform bootstrap as a SORACOM Inventory device
		generateAmazonCognitoSessionCredentials   generates AWS temporary session token using Amazon Cognito
		generateAmazonCognitoOpenIdToken          generates an Open ID Token using Amazon Cognito
	*/

	return `Choose which type of provisioning API will be performed. (required)
Possible values:
` + s

}

func getMaxLength(ss []string) int {
	n := 0
	for _, s := range ss {
		l := len(s)
		if l > n {
			n = l
		}
	}
	return n
}
