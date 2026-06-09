#include "config/relay-settings.hpp"

#include "config/credential-store.hpp"
#include "config/relay-destination-url.hpp"
#include "dock/delaydeck-dock.hpp"

#include <obs-frontend-api.h>
#include <obs.hpp>

#include <QProcessEnvironment>
#include <QTimer>

namespace delaydeck {

namespace {

QString envOrDefault(const char *name, const QString &fallback = {})
{
	const auto value =
		QProcessEnvironment::systemEnvironment().value(QString::fromUtf8(name));
	return value.isEmpty() ? fallback : value;
}

bool envForwardingConfigured()
{
	return !envOrDefault("DELAYDECK_OUTPUT_URL").isEmpty() &&
	       !envOrDefault("DELAYDECK_OUTPUT_STREAM_KEY").isEmpty();
}

DelayDeckDock *registered_dock_ = nullptr;

void scheduleRelayStartupSync()
{
	if (!registered_dock_) {
		return;
	}

	QTimer::singleShot(0, registered_dock_, []() {
		if (registered_dock_) {
			registered_dock_->syncManagedRelayStartup();
		}
	});
}

void relaySettingsCallback(obs_data_t *save_data, bool saving, void *)
{
	if (saving) {
		RelaySettings::instance().saveToObsData(save_data);
		return;
	}

	RelaySettings::instance().loadFromObsData(save_data);
	scheduleRelayStartupSync();
}

} // namespace

RelaySettings &RelaySettings::instance()
{
	static RelaySettings settings;
	return settings;
}

bool RelaySettings::isSetupComplete() const
{
	if (envForwardingConfigured()) {
		return true;
	}
	return setup_complete_ && !output_url_.trimmed().isEmpty();
}

RelayDestination RelaySettings::destination() const
{
	RelayDestination dest;

	const QString envUrl = envOrDefault("DELAYDECK_OUTPUT_URL");
	const QString envKey = envOrDefault("DELAYDECK_OUTPUT_STREAM_KEY");
	if (!envUrl.isEmpty() && !envKey.isEmpty()) {
		dest.outputUrl = envUrl;
		dest.streamKey = envKey;
		return dest;
	}

	dest.outputUrl = output_url_.trimmed();
	readStreamKey(&dest.streamKey);
	return dest;
}

bool RelaySettings::saveDestination(const QString &outputUrl,
				    const QString &streamKey)
{
	const QString trimmedUrl = outputUrl.trimmed();
	const QString trimmedKey = streamKey.trimmed();
	if (trimmedUrl.isEmpty() || trimmedKey.isEmpty()) {
		return false;
	}

	if (!isValidRelayOutputUrl(trimmedUrl)) {
		return false;
	}

	if (!writeStreamKey(trimmedKey)) {
		return false;
	}

	output_url_ = trimmedUrl;
	setup_complete_ = true;
	obs_frontend_save();
	return true;
}

void RelaySettings::clearDestination()
{
	deleteStreamKey();
	output_url_.clear();
	setup_complete_ = false;
	obs_frontend_save();
}

void RelaySettings::loadFromObsData(obs_data_t *data)
{
	if (!data) {
		return;
	}

	OBSDataAutoRelease obj = obs_data_get_obj(data, kRelaySettingsKey);
	if (!obj) {
		return;
	}

	output_url_ = QString::fromUtf8(obs_data_get_string(obj, "output_url"));
	setup_complete_ = obs_data_get_bool(obj, "setup_complete");
}

void RelaySettings::saveToObsData(obs_data_t *data) const
{
	if (!data) {
		return;
	}

	OBSDataAutoRelease obj = obs_data_create();
	obs_data_set_string(obj, "output_url",
			    output_url_.toUtf8().constData());
	obs_data_set_bool(obj, "setup_complete", setup_complete_);
	obs_data_set_obj(data, kRelaySettingsKey, obj);
}

void registerRelaySettingsCallbacks(DelayDeckDock *dock)
{
	registered_dock_ = dock;
	obs_frontend_add_save_callback(relaySettingsCallback, nullptr);
}

void unregisterRelaySettingsCallbacks()
{
	obs_frontend_remove_save_callback(relaySettingsCallback, nullptr);
	registered_dock_ = nullptr;
}

} // namespace delaydeck
