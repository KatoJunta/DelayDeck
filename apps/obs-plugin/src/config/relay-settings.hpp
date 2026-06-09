#pragma once

#include <obs.h>

#include <QString>

class DelayDeckDock;

namespace delaydeck {

constexpr const char *kRelaySettingsKey = "delaydeck-relay";
constexpr const char *kDefaultIngestListen = "127.0.0.1:9401";
constexpr const char *kLocalRelayServerUrl = "rtmp://127.0.0.1:9401/live";
constexpr const char *kLocalRelayStreamKey = "stream";

struct RelayDestination {
	QString outputUrl;
	QString streamKey;

	bool isComplete() const
	{
		return !outputUrl.trimmed().isEmpty() &&
		       !streamKey.trimmed().isEmpty();
	}
};

class RelaySettings {
public:
	static RelaySettings &instance();

	bool isSetupComplete() const;
	RelayDestination destination() const;

	bool saveDestination(const QString &outputUrl, const QString &streamKey);
	void clearDestination();

	void loadFromObsData(obs_data_t *data);
	void saveToObsData(obs_data_t *data) const;

private:
	RelaySettings() = default;

	QString output_url_;
	bool setup_complete_ = false;
};

void registerRelaySettingsCallbacks(DelayDeckDock *dock);
void unregisterRelaySettingsCallbacks();

} // namespace delaydeck
