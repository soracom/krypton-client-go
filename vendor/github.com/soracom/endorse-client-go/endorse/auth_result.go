package endorse

type AuthenticationResult struct {
	KeyID string `json:"keyId"`
	IMSI  string `json:"imsi"`
	CK    []byte `json:"ck"`
}
