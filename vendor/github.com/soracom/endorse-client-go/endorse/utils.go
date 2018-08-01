package endorse

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/pkg/errors"
)

func doesFileExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func readUint32(r io.Reader) (uint32, error) {
	var v uint32
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

func readInt64(r io.Reader) (int64, error) {
	var v int64
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

func readUTF8(r io.Reader, l int) (string, error) {
	v := make([]byte, l)
	err := binary.Read(r, binary.BigEndian, &v)
	return string(v), err
}

func readBytes(r io.Reader, l int) ([]byte, error) {
	v := make([]byte, l)
	err := binary.Read(r, binary.BigEndian, &v)
	return v, err
}

func writeUint32(w io.Writer, v uint32) error {
	return binary.Write(w, binary.BigEndian, v)
}

func writeInt64(w io.Writer, v int64) error {
	return binary.Write(w, binary.BigEndian, v)
}

func writeUTF8(w io.Writer, v string) error {
	nWritten, err := w.Write([]byte(v))
	if nWritten != len(v) {
		return errors.New("unable to write whole string")
	}
	return err
}

func writeBytes(w io.Writer, v []byte) error {
	nWritten, err := w.Write(v)
	if nWritten != len(v) {
		return errors.New("unable to write whole bytes")
	}
	return err
}
