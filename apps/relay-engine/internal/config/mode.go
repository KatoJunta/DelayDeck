package config

type RunMode string

const RunModeForwarding RunMode = "forwarding"

func (m RunMode) Valid() bool {
	return m == RunModeForwarding
}

func (m RunMode) String() string {
	return string(m)
}
