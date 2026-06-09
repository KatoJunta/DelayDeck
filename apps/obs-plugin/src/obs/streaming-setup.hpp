#pragma once

#include <QString>

namespace delaydeck {

struct ObsStreamingSnapshot {
	QString serviceType;
	QString serverUrl;
	QString streamKey;

	bool isLocalRelay() const;
	bool canPrefillDestination() const;
};

ObsStreamingSnapshot readCurrentStreaming();
bool applyLocalRelayIngest();
bool isConfiguredForLocalRelay();

} // namespace delaydeck
