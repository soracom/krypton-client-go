package endorse

import "github.com/pkg/errors"

type UICCAuthResult struct {
	Status AuthStatus
	AUTS   []byte
	RES    []byte
	CK     []byte
	IK     []byte
	KC     []byte
}

func parseUICCAuthResult(rsp []byte) (*UICCAuthResult, error) {
	if rsp[0] == 0xdc {
		l := rsp[1]
		return &UICCAuthResult{
			Status: AuthStatusSynchronisationFailure,
			AUTS:   rsp[2 : 2+l],
		}, nil
	} else if rsp[0] == 0xdb {
		rsp = rsp[1:]
		l := rsp[0]
		res := rsp[1 : 1+l]
		rsp = rsp[1+l:]
		l = rsp[0]
		ck := rsp[1 : 1+l]
		rsp = rsp[1+l:]
		l = rsp[0]
		ik := rsp[1 : 1+l]
		rsp = rsp[1+l:]
		l = rsp[0]
		kc := rsp[1 : 1+l]
		rsp = rsp[1+l:]

		return &UICCAuthResult{
			Status: AuthStatusSuccess,
			RES:    res,
			CK:     ck,
			IK:     ik,
			KC:     kc,
		}, nil
	}

	return &UICCAuthResult{
		Status: AuthStatusSynchronisationFailure,
	}, errors.New("unable to authenticate")
}
