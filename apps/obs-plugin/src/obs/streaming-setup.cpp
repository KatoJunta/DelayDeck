#include "obs/streaming-setup.hpp"

#include "config/relay-settings.hpp"

#include <obs-frontend-api.h>
#include <obs-module.h>
#include <obs.hpp>

#include <QHostAddress>
#include <QUrl>

namespace delaydeck {

namespace {

bool isLoopbackHost(const QString &host)
{
	const QString normalized = host.trimmed().toLower();
	if (normalized == QStringLiteral("localhost") ||
	    normalized == QStringLiteral("127.0.0.1") ||
	    normalized == QStringLiteral("::1")) {
		return true;
	}

	const QHostAddress address(normalized);
	return address.isLoopback();
}

bool serverMatchesLocalIngest(const QString &serverUrl)
{
	const QUrl url(serverUrl.trimmed());
	if (!url.isValid() || url.host().isEmpty()) {
		return false;
	}
	if (!isLoopbackHost(url.host())) {
		return false;
	}

	const int port = url.port(9401);
	return port == 9401;
}

} // namespace

bool ObsStreamingSnapshot::isLocalRelay() const
{
	return serverMatchesLocalIngest(serverUrl);
}

bool ObsStreamingSnapshot::canPrefillDestination() const
{
	return !serverUrl.trimmed().isEmpty() && !streamKey.trimmed().isEmpty() &&
	       !isLocalRelay();
}

ObsStreamingSnapshot readCurrentStreaming()
{
	ObsStreamingSnapshot snapshot;

	OBSService service = obs_frontend_get_streaming_service();
	if (!service) {
		return snapshot;
	}

	snapshot.serviceType =
		QString::fromUtf8(obs_service_get_type(service));

	OBSDataAutoRelease settings = obs_service_get_settings(service);
	if (settings) {
		const char *server = obs_data_get_string(settings, "server");
		if (server && server[0] != '\0') {
			snapshot.serverUrl = QString::fromUtf8(server);
		}
		const char *key = obs_data_get_string(settings, "key");
		if (key && key[0] != '\0') {
			snapshot.streamKey = QString::fromUtf8(key);
		}
	}

	const char *connectServer = obs_service_get_connect_info(
		service, OBS_SERVICE_CONNECT_INFO_SERVER_URL);
	if (connectServer && connectServer[0] != '\0') {
		snapshot.serverUrl = QString::fromUtf8(connectServer);
	}

	const char *connectKey = obs_service_get_connect_info(
		service, OBS_SERVICE_CONNECT_INFO_STREAM_KEY);
	if (connectKey && connectKey[0] != '\0') {
		snapshot.streamKey = QString::fromUtf8(connectKey);
	}

	return snapshot;
}

bool applyLocalRelayIngest()
{
	if (obs_frontend_streaming_active()) {
		return false;
	}

	OBSDataAutoRelease settings = obs_data_create();
	obs_data_set_string(settings, "server", kLocalRelayServerUrl);
	obs_data_set_string(settings, "key", kLocalRelayStreamKey);

	OBSService current = obs_frontend_get_streaming_service();
	const char *currentType =
		current ? obs_service_get_type(current) : nullptr;

	if (currentType &&
	    strcmp(currentType, "rtmp_custom") == 0) {
		obs_service_update(current, settings);
	} else {
		OBSServiceAutoRelease service = obs_service_create(
			"rtmp_custom", "delaydeck_local_relay", settings,
			nullptr);
		if (!service) {
			return false;
		}
		obs_frontend_set_streaming_service(service);
	}

	obs_frontend_save_streaming_service();
	return true;
}

bool isConfiguredForLocalRelay()
{
	return readCurrentStreaming().isLocalRelay();
}

} // namespace delaydeck
