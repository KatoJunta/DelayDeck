package config

type RunMode string

const (
	RunModeMock       RunMode = "mock"
	RunModeForwarding RunMode = "forwarding"
)

func (m RunMode) Valid() bool {
	return m == RunModeMock || m == RunModeForwarding
}

func (m RunMode) String() string {
	return string(m)
}
