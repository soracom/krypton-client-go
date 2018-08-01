package endorse

type challenge struct {
	KeyID string `json:"keyId"`
	RAND  []byte `json:"rand"`
	AUTN  []byte `json:"autn"`
}
