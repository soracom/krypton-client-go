package endorse

type noOpKeyCache struct {
}

func (kc *noOpKeyCache) findAuthResult(imsi string) (*AuthenticationResult, error) {
	return nil, nil
}

func (kc *noOpKeyCache) saveAuthResult(imsi string, ar *AuthenticationResult) error {
	return nil
}
