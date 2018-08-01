package endorse

import (
	"encoding/hex"

	"github.com/ebfe/scard"
	"github.com/pkg/errors"
)

const (
	CLA_UICC                  = 0x00
	INS_INTERNAL_AUTHENTICATE = 0x88
	INS_SELECT                = 0xa4
	INS_READ_BINARY           = 0xb0
	INS_READ_RECORD           = 0xb2
	INS_GET_RESPONSE          = 0xc0
)

var (
	FID_MF      = []byte{0x3f, 0x00}
	FID_EF_DIR  = []byte{0x2f, 0x00}
	FID_EF_IMSI = []byte{0x6f, 0x07}
)

type iso7816 struct {
	name    string
	ctx     *scard.Context
	card    *scard.Card
	adfUSIM []byte
}

func newISO7816(index int) (*iso7816, error) {
	ctx, err := scard.EstablishContext()
	if err != nil {
		return nil, errors.Wrap(err, "unable to establish context")
	}

	readers, err := ctx.ListReaders()
	if err != nil {
		return nil, errors.Wrap(err, "unable to list readers")
	}

	if len(readers) < index {
		return nil, errors.New("no smartcard readers found at the specified index")
	}

	reader := readers[index]
	card, err := ctx.Connect(reader, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect to the card")
	}

	return &iso7816{
		name: reader,
		ctx:  ctx,
		card: card,
	}, nil
}

func (i *iso7816) Close() {
	i.card.Disconnect(scard.LeaveCard)
	i.ctx.Release()
}

func (i *iso7816) String() string {
	return i.name
}

func (i *iso7816) ReadIMSI() (string, error) {
	var err error
	if i.adfUSIM == nil {
		i.adfUSIM, err = i.findADFUSIM()
		if err != nil {
			return "", err
		}
	}

	cmdSelectADFUSIM := newCommandAPDUForSelect(0x04, 0x04, i.adfUSIM)
	rsp, err := i.card.Transmit(cmdSelectADFUSIM)
	if err != nil {
		return "", err
	}

	if !isSuccessfulSW(rsp) {
		return "", errors.New("unsuccessful response for SELECT ADF RECORD")
	}

	log("response for SELECT (ADF record) == %s\n", hex.EncodeToString(rsp))

	cmdSelectIMSI := newCommandAPDUForSelect(0x00, 0x04, FID_EF_IMSI)
	rsp, err = i.card.Transmit(cmdSelectIMSI)
	if err != nil {
		return "", err
	}

	if !isSuccessfulSW(rsp) {
		return "", errors.New("unsuccessful response for SELECT EF_IMSI")
	}

	log("response for SELECT (EF_IMSI) == %s\n", hex.EncodeToString(rsp))

	cmdReadBinaryIMSI := newCommandAPDUForReadBinary(0x00, 0x00, 9)
	rsp, err = i.card.Transmit(cmdReadBinaryIMSI)
	if err != nil {
		return "", err
	}

	if !isSuccessfulSW(rsp) {
		return "", errors.New("unsuccessful response for READ BINARY EF_IMSI")
	}

	log("response for READ BINARY (EF_IMSI) == %s\n", hex.EncodeToString(rsp))

	imsi := decodeTBCD(rsp[1 : rsp[0]+1])
	return imsi[1:], nil
}

func (i *iso7816) Authenticate(rand []byte, autn []byte) (*UICCAuthResult, error) {
	var err error
	if i.adfUSIM == nil {
		i.adfUSIM, err = i.findADFUSIM()
		if err != nil {
			return nil, err
		}
	}

	// P1 = 0x04: selection by path, select from the MF
	// P2 = 0x04:
	//    File control information: Return FMD template, mandatory use of FMD tag and length
	//    File occurrence: First or only occurrence
	cmdSelectADFUSIM := newCommandAPDUForSelect(0x04, 0x04, i.adfUSIM)
	rsp, err := i.card.Transmit(cmdSelectADFUSIM)
	if err != nil {
		return nil, errors.Wrap(err, "error occurred while sending command: SELECT ADF USIM")
	}

	if !isSuccessfulSW(rsp) {
		return nil, errors.New("unsuccessful response for SELECT ADF")
	}

	// P1, P2 = 0x00 0x00 (any other value is reserved for future use)
	cmdGetResponseDir := newCommandAPDUForGetResponse(0x00, 0x00, rsp[len(rsp)-1])
	rsp, err = i.card.Transmit(cmdGetResponseDir)
	if err != nil {
		return nil, errors.Wrap(err, "error occurred while sending command: GET RESPONSE")
	}

	if !isSuccessfulSW(rsp) {
		return nil, errors.New("unsuccessful response for GET RESPONSE")
	}

	log("rsp == %s\n", hex.EncodeToString(rsp))
	//fcp, err := parseFCP(rsp)
	//if err != nil {
	//return nil, err
	//}

	// P1 = 0x00: no information is given
	// P2 = 0x81:
	//    Specific reference data (e.g., DF specific password or key)
	//    Qualifier = 1 (i.e., number of the reference data or number of the secret)
	cmdAuthenticate := newCommandAPDUForAuthenticate(0x00, 0x81, rand, autn)
	rsp, err = i.card.Transmit(cmdAuthenticate)
	if err != nil {
		return nil, errors.Wrap(err, "error occurred while sending command: AUTHENTICATE")
	}

	log("response for INTERNAL AUTHENTICATE == %s\n", hex.EncodeToString(rsp))
	sw1 := rsp[len(rsp)-2]
	if sw1 != 0x61 && sw1 != 0x6e {
		return nil, errors.New("unsuccessful response for AUTHENTICATE")
	}

	// P1, P2 = 0x00 0x00 (any other value is reserved for future use)
	cmdGetResponse := newCommandAPDUForGetResponse(0x00, 0x00, rsp[len(rsp)-1])
	rsp, err = i.card.Transmit(cmdGetResponse)
	if err != nil {
		return nil, errors.Wrap(err, "error occurred while sending command: GET RESPONSE")
	}

	if !isSuccessfulSW(rsp) {
		return nil, errors.New("unsuccessful response for GET RESPONSE")
	}

	log("rsp == %s\n", hex.EncodeToString(rsp))
	return parseUICCAuthResult(rsp)
}

func (i *iso7816) findADFUSIM() ([]byte, error) {
	// P1 = 0x00 : select MF, DF or EF by file identifier
	// P2 = 0x04 :
	//    File control information: Return FMD template, mandatory use of FMD tag and length
	//    File occurrence: First or only occurence
	cmdSelectMF := newCommandAPDUForSelect(0x00, 0x04, FID_MF)
	rsp, err := i.card.Transmit(cmdSelectMF)
	if err != nil {
		return nil, err
	}

	if !isSuccessfulSW(rsp) {
		return nil, errors.Errorf("unsuccessful response for SELECT MF: %s", hex.EncodeToString(rsp))
	}

	// P1 = 0x00 : select MF, DF or EF by file identifier
	// P2 = 0x04 :
	//    File control information: Return FMD template, mandatory use of FMD tag and length
	//    File occurrence: First or only occurence
	cmdSelectDir := newCommandAPDUForSelect(0x00, 0x04, FID_EF_DIR)
	rsp, err = i.card.Transmit(cmdSelectDir)
	if err != nil {
		return nil, err
	}

	if !isSuccessfulSW(rsp) {
		return nil, errors.New("unsuccessful response for SELECT EF_DIR")
	}

	if rsp[len(rsp)-2] != 0x61 {
		return nil, errors.New("unwanted response")
	}

	// P1, P2 = 0x00 0x00 (any other value is reserved for future use)
	cmdGetResponseDir := newCommandAPDUForGetResponse(0x00, 0x00, rsp[len(rsp)-1])
	rsp, err = i.card.Transmit(cmdGetResponseDir)
	if err != nil {
		return nil, err
	}

	if !isSuccessfulSW(rsp) {
		return nil, errors.New("unsuccessful response for GET RESPONSE")
	}

	log("rsp == %s\n", hex.EncodeToString(rsp))
	fcp, err := parseFCP(rsp)
	if err != nil {
		return nil, err
	}

	recSize := fcp.fileDescriptor.getRecordSizeBytes()

	// P1 = 0x01: Record number or record identifier
	// P2 = 0x04: Read record number in P1
	cmdReadRecordADFUSIM := newCommandAPDUForReadRecord(0x01, 0x04, recSize[1])
	rsp, err = i.card.Transmit(cmdReadRecordADFUSIM)
	if err != nil {
		return nil, err
	}

	if !isSuccessfulSW(rsp) {
		return nil, errors.New("unsuccessful response for READ ADF RECORD")
	}

	log("ADF record == %s\n", hex.EncodeToString(rsp))

	dir, err := parseApplicationTemplate(rsp)
	if err != nil {
		return nil, err
	}

	log("Application identifier == %s\n", hex.EncodeToString(dir.applicationIdentifier))
	i.adfUSIM = dir.applicationIdentifier
	return dir.applicationIdentifier, nil
}

func isSuccessfulSW(rsp []byte) bool {
	if len(rsp) < 2 {
		return false
	}

	sw1 := rsp[len(rsp)-2]
	switch sw1 {
	case 0x61, 0x6f, 0x90:
		return true
	}

	return false
}

func decodeTBCD(bytes []byte) string {
	swapped := make([]byte, len(bytes))
	log("bytes == %s\n", hex.EncodeToString(bytes))
	for i, b := range bytes {
		swapped[i] = ((b & 0xf) << 4) | ((b & 0xf0) >> 4)
	}
	return hex.EncodeToString(swapped)
}

type fileDescriptor []byte

type fcp struct {
	fileDescriptor         fileDescriptor
	fileIdentifier         []byte
	lifeCycleStatusInteger byte
	securityAttribute1     []byte
	fileSize               uint16
	shortFileIdentifier    byte
}

func parseFCP(b []byte) (*fcp, error) {
	if len(b) < 2 {
		return nil, errors.New("too short")
	}

	if b[0] != 0x62 {
		return nil, errors.New("the first byte is not TAG_FCP_TEMPLATE")
	}

	totalLen := b[1]
	if len(b) < int(totalLen+2) {
		return nil, errors.New("too short")
	}

	b = b[2 : 2+totalLen]

	var fcp fcp
	for {
		tag := b[0]
		l := b[1]
		switch tag {
		case 0x80:
			if l != 2 {
				return nil, errors.New("unsupported file size length")
			}
			fcp.fileSize = readUint16LE(b[2:4])

		case 0x82:
			fcp.fileDescriptor = b[2 : l+2]
		case 0x83:
			fcp.fileIdentifier = b[2 : l+2]
		case 0x88:
			if l == 1 {
				fcp.shortFileIdentifier = b[2]
			}
		case 0x8a:
			fcp.lifeCycleStatusInteger = b[2]
		case 0x8b:
			fcp.securityAttribute1 = b[2 : l+2]
		}

		b = b[l+2:]

		if len(b) < 2 {
			return &fcp, nil
		}
	}
}

func readUint16LE(b []byte) uint16 {
	if len(b) < 2 {
		panic("length must be larger than 2 bytes")
	}
	return (uint16(b[0]) << 8) | uint16(b[1])
}

func (fd fileDescriptor) getRecordSizeBytes() []byte {
	return fd[2:4]
}

type dir struct {
	applicationIdentifier []byte
	applicationLabel      []byte
}

func parseApplicationTemplate(b []byte) (*dir, error) {
	if len(b) < 2 {
		return nil, errors.New("too short")
	}

	if b[0] != 0x61 {
		return nil, errors.New("the first byte is not TAG_APPLICATION_TEMPLATE")
	}

	totalLen := b[1]
	if len(b) < int(totalLen+2) {
		return nil, errors.New("too short")
	}

	b = b[2 : 2+totalLen]

	var dir dir
	for {
		tag := b[0]
		l := b[1]
		switch tag {
		case 0x4f:
			dir.applicationIdentifier = b[2 : l+2]
		case 0x50:
			dir.applicationLabel = b[2 : l+2]
		}

		b = b[l+2:]

		if len(b) < 2 {
			return &dir, nil
		}
	}
}

type commandAPDU []byte

func newCommandAPDUForSelect(p1, p2 byte, b []byte) commandAPDU {
	return append(commandAPDU{CLA_UICC, INS_SELECT, p1, p2, byte(len(b))}, b...)
}

func newCommandAPDUForGetResponse(p1, p2 byte, l byte) commandAPDU {
	return commandAPDU{CLA_UICC, INS_GET_RESPONSE, p1, p2, l}
}

func newCommandAPDUForReadRecord(p1, p2 byte, l byte) commandAPDU {
	return commandAPDU{CLA_UICC, INS_READ_RECORD, p1, p2, l}
}

func newCommandAPDUForReadBinary(p1, p2 byte, l byte) commandAPDU {
	return commandAPDU{CLA_UICC, INS_READ_BINARY, p1, p2, l}
}

func newCommandAPDUForAuthenticate(p1, p2 byte, rand, autn []byte) commandAPDU {
	a := append(commandAPDU{CLA_UICC, INS_INTERNAL_AUTHENTICATE, p1, p2, byte(len(rand) + len(autn) + 2), byte(len(rand))}, rand...)
	a = append(a, byte(len(autn)))
	a = append(a, autn...)
	return a
}
