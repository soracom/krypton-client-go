package endorse

type AuthStatus int

const (
	AuthStatusSuccess AuthStatus = iota
	AuthStatusSynchronisationFailure
)
