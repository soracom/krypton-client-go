package endorse

import (
	"bytes"
	"crypto/hmac"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type keyCacheImpl struct {
	ks map[string]keyCacheEntry
}

func newKeyCacheImpl() *keyCacheImpl {
	return &keyCacheImpl{
		ks: make(map[string]keyCacheEntry),
	}
}

func (kc *keyCacheImpl) loadFromFile(path string, password []byte) error {
	dir := filepath.Dir(path)

	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return kc.load(f, password)
}

func (kc *keyCacheImpl) load(originalReader io.Reader, password []byte) error {
	bufSig := bytes.NewBuffer([]byte{})
	r := io.TeeReader(originalReader, bufSig)

	// load magic
	magic, err := readUint32(r)
	if err != nil {
		return err
	}
	if magic != magicKeyCache {
		return errors.New("magic number does not match")
	}

	// load version
	ver, err := readUint32(r)
	if err != nil {
		return err
	}
	if !isSupportedKeyCacheVersion(ver) {
		return errors.New("unsupported version number")
	}

	// load entry count
	n, err := readUint32(r)
	if err != nil {
		return err
	}

	ks := make(map[string]keyCacheEntry)

	// load entries
	for i := uint32(0); i < n; i++ {
		// read tag
		tag, err := readUint32(r)
		if err != nil {
			return err
		}

		if tag != tagSecretKeyEntry {
			return errors.New("unsupported tag")
		}

		// read length of alias
		la, err := readUint32(r)
		if err != nil {
			return err
		}

		// read alias
		alias, err := readUTF8(r, int(la))
		if err != nil {
			return err
		}

		// read length of encoded value
		le, err := readUint32(r)
		if err != nil {
			return err
		}

		// read value
		encodedData, err := readBytes(r, int(le))
		if err != nil {
			return err
		}

		// decode entry
		decodedData, err := decode(encodedData, password)
		if err != nil {
			return err
		}

		e, err := parseKeyCacheEntry(decodedData)
		if err != nil {
			return err
		}

		ks[alias] = *e
	}

	// calculate signature
	sigMine := calculateSignature(bufSig.Bytes(), password)

	// load signature
	sigTheirs, err := readBytes(originalReader, len(sigMine))
	if err != nil {
		return err
	}

	// verify signature
	if !hmac.Equal(sigMine, sigTheirs) {
		return errors.New("signature does not match")
	}

	kc.ks = ks
	return nil
}

func (kc *keyCacheImpl) saveToFile(path string, password []byte) error {
	dir := filepath.Dir(path)

	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	return kc.save(f, password)
}

func (kc *keyCacheImpl) save(originalWriter io.Writer, password []byte) error {
	var bufSig bytes.Buffer
	w := io.MultiWriter(originalWriter, &bufSig)

	// write magic
	err := writeUint32(w, magicKeyCache)
	if err != nil {
		return err
	}

	// write version
	err = writeUint32(w, currentKeyCacheVersion)
	if err != nil {
		return err
	}

	// write entry count
	n := uint32(len(kc.ks))
	err = writeUint32(w, n)
	if err != nil {
		return err
	}

	// write entries
	for alias, entry := range kc.ks {
		// write tag
		err = writeUint32(w, tagSecretKeyEntry)
		if err != nil {
			return err
		}

		// write length of alias
		err = writeUint32(w, uint32(len(alias)))
		if err != nil {
			return err
		}

		// write alias
		err = writeUTF8(w, alias)
		if err != nil {
			return err
		}

		encodedEntry, err := entry.serialize()
		if err != nil {
			return err
		}

		iv := generateRandomIV()
		b, err := encode(encodedEntry, password, iv)
		if err != nil {
			return err
		}

		// write length of value
		err = writeUint32(w, uint32(len(b)))
		if err != nil {
			return err
		}

		// write encoded value
		err = writeBytes(w, b)
		if err != nil {
			return err
		}
	}

	// write signature
	sig := calculateSignature(bufSig.Bytes(), password)
	err = writeBytes(originalWriter, sig)
	if err != nil {
		return err
	}

	return nil
}

func (kc *keyCacheImpl) findAuthResult(imsi string) (*AuthenticationResult, error) {
	for alias, e := range kc.ks {
		if e.isExpired() {
			kc.unset(alias)
			continue
		}
		if !strings.HasPrefix(alias, imsi) {
			kc.unset(alias)
			continue
		}
		s := strings.Split(alias, "_")
		if len(s) < 2 {
			kc.unset(alias)
			continue
		}

		return &AuthenticationResult{
			KeyID: s[1],
			IMSI:  imsi,
			CK:    e.key,
		}, nil
	}
	return nil, nil
}

func (kc *keyCacheImpl) saveAuthResult(imsi string, ar *AuthenticationResult) error {
	alias := fmt.Sprintf("%s_%s", imsi, ar.KeyID)
	kc.ks[alias] = keyCacheEntry{
		created: time.Now(),
		key:     ar.CK,
	}

	password := loadKeyCachePassword()
	path := getKeyCachePath()
	err := kc.saveToFile(path, password)
	if err != nil {
		return err
	}

	return nil
}

func (kc *keyCacheImpl) unset(alias string) {
	delete(kc.ks, alias)
}
