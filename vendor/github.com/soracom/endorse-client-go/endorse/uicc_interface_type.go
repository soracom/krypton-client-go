package endorse

import (
	"strings"

	"github.com/pkg/errors"
)

type UICCInterfaceType int

const (
	UICCInterfaceTypeISO7816 UICCInterfaceType = iota
	UICCInterfaceTypeComm
	UICCInterfaceTypeAutoDetect
	UICCInterfaceTypeNone
)

func ParseUICCInterfaceType(s string) (*UICCInterfaceType, error) {
	s = strings.ToLower(s)
	if s == "iso7816" {
		return &[]UICCInterfaceType{UICCInterfaceTypeISO7816}[0], nil
	}
	if s == "comm" {
		return &[]UICCInterfaceType{UICCInterfaceTypeComm}[0], nil
	}
	if s == "autodetect" {
		return &[]UICCInterfaceType{UICCInterfaceTypeAutoDetect}[0], nil
	}

	return nil, errors.Errorf("unknown UICC interface type: %s", s)
}
