#pragma once

#include <QString>

enum class PreflightFailureCode {
	None,
	RelayProcessNotRunning,
	RelayUnreachable,
	RelayUnhealthy,
	IngestNotListening,
	DestinationNotConfigured,
	DestinationNotLocalRelay,
	ObsNativeDelayEnabled,
};

struct PreflightResult {
	bool ok = false;
	PreflightFailureCode code = PreflightFailureCode::None;
	QString detail;
};
