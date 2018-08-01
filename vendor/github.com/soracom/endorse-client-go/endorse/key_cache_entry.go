package endorse

import (
	"bytes"
	"encoding/gob"
	"time"
)

type keyCacheEntry struct {
	created time.Time
	key     []byte
}

func (e *keyCacheEntry) isExpired() bool {
	expiry := e.created.Add(keyValidDuration)
	return time.Now().After(expiry)
}

func (e *keyCacheEntry) serialize() ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	enc := gob.NewEncoder(buf)
	err := enc.Encode(e.created)
	if err != nil {
		return nil, err
	}
	err = enc.Encode(e.key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func parseKeyCacheEntry(b []byte) (*keyCacheEntry, error) {
	r := bytes.NewReader(b)
	dec := gob.NewDecoder(r)
	e := keyCacheEntry{}
	err := dec.Decode(&e.created)
	if err != nil {
		return nil, err
	}
	err = dec.Decode(&e.key)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
