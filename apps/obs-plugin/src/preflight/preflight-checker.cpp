#include "preflight-checker.hpp"

#include "relay/relay-types.hpp"

#include <obs-frontend-api.h>
#include <obs-module.h>
#include <obs.hpp>
#include <util/config-file.h>

#include <QEventLoop>
#include <QHostAddress>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QProcessEnvironment>
#include <QTcpSocket>
#include <QTimer>
#include <QUrl>

namespace {

constexpr int kHealthTimeoutMs = 4000;
constexpr int kIngestProbeTimeoutMs = 2000;

QString envOrDefault(const char *name, const QString &fallback)
{
	const auto value =
		QProcessEnvironment::systemEnvironment().value(QString::fromUtf8(name));
	return value.isEmpty() ? fallback : value;
}

bool envFlagEnabled(const char *name, bool defaultValue)
{
	const auto value = envOrDefault(name, QString());
	if (value.isEmpty()) {
		return defaultValue;
	}
	const auto lower = value.toLower();
	return lower != QStringLiteral("0") && lower != QStringLiteral("false") &&
	       lower != QStringLiteral("no");
}

PreflightResult fail(PreflightFailureCode code, const QString &detail)
{
	return PreflightResult{false, code, detail};
}

} // namespace

bool PreflightChecker::delayDeckModeEnabled()
{
	return envFlagEnabled("DELAYDECK_MODE", true);
}

bool PreflightChecker::ingestEndpoint(QString *host, quint16 *port)
{
	const auto endpoint =
		envOrDefault("DELAYDECK_INGEST_LISTEN",
			     QStringLiteral("127.0.0.1:9401"));
	const QStringList parts = endpoint.split(QLatin1Char(':'));
	if (parts.size() != 2) {
		return false;
	}

	bool ok = false;
	const int parsedPort = parts[1].toInt(&ok);
	if (!ok || parsedPort <= 0 || parsedPort > 65535) {
		return false;
	}

	*host = parts[0].trimmed();
	*port = static_cast<quint16>(parsedPort);
	return !host->isEmpty();
}

bool PreflightChecker::isLoopbackHost(const QString &host)
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

bool PreflightChecker::probeTcpEndpoint(const QString &host, quint16 port,
					int timeoutMs)
{
	QTcpSocket socket;
	socket.connectToHost(host, port);
	if (!socket.waitForConnected(timeoutMs)) {
		return false;
	}
	socket.disconnectFromHost();
	return true;
}

bool PreflightChecker::fetchRelayHealthy(const QString &apiBaseUrl,
					 QString *detail)
{
	QUrl url(apiBaseUrl);
	if (!url.isValid()) {
		*detail = QStringLiteral("invalid relay API URL");
		return false;
	}
	url.setPath(QStringLiteral("/v1/health"));

	QNetworkAccessManager nam;
	QEventLoop loop;
	QTimer timeout;
	timeout.setSingleShot(true);

	QNetworkReply *reply = nam.get(QNetworkRequest(url));
	QObject::connect(reply, &QNetworkReply::finished, &loop, &QEventLoop::quit);
	QObject::connect(&timeout, &QTimer::timeout, &loop, &QEventLoop::quit);
	timeout.start(kHealthTimeoutMs);
	loop.exec();

	if (!reply->isFinished()) {
		reply->abort();
		*detail = QStringLiteral("relay health request timed out");
		return false;
	}

	if (reply->error() != QNetworkReply::NoError) {
		*detail = reply->errorString();
		return false;
	}

	const auto document =
		QJsonDocument::fromJson(reply->readAll());
	reply->deleteLater();

	if (!document.isObject()) {
		*detail = QStringLiteral("relay health response is not JSON");
		return false;
	}

	const RelayHealth health = parseHealth(document.object());
	if (!health.healthy) {
		*detail = QStringLiteral("relay reported unhealthy");
		return false;
	}

	return true;
}

PreflightResult PreflightChecker::checkObsDestination(const QString &expectedHost,
						      quint16 expectedPort)
{
	OBSService service = obs_frontend_get_streaming_service();
	if (!service) {
		return fail(PreflightFailureCode::DestinationNotConfigured,
			    QStringLiteral("streaming service is not configured"));
	}

	const char *serverUrl =
		obs_service_get_connect_info(service,
					     OBS_SERVICE_CONNECT_INFO_SERVER_URL);
	if (!serverUrl || serverUrl[0] == '\0') {
		return fail(PreflightFailureCode::DestinationNotConfigured,
			    QStringLiteral("stream server URL is empty"));
	}

	const QUrl url(QString::fromUtf8(serverUrl));
	if (!url.isValid() || url.host().isEmpty()) {
		return fail(PreflightFailureCode::DestinationNotLocalRelay,
			    QStringLiteral("stream server URL is invalid"));
	}

	if (!isLoopbackHost(url.host())) {
		return fail(PreflightFailureCode::DestinationNotLocalRelay,
			    QStringLiteral("stream server must use localhost"));
	}

	const int port = url.port(expectedPort);
	if (port != expectedPort) {
		return fail(PreflightFailureCode::DestinationNotLocalRelay,
			    QStringLiteral("stream server port must be %1")
				    .arg(expectedPort));
	}

	const QString expectedHostNormalized = expectedHost.trimmed().toLower();
	const QString actualHost = url.host().trimmed().toLower();
	if (actualHost != expectedHostNormalized &&
	    actualHost != QStringLiteral("localhost") &&
	    actualHost != QStringLiteral("127.0.0.1") &&
	    actualHost != QStringLiteral("::1")) {
		return fail(PreflightFailureCode::DestinationNotLocalRelay,
			    QStringLiteral("stream server host must be %1")
				    .arg(expectedHost));
	}

	return PreflightResult{true, PreflightFailureCode::None, {}};
}

PreflightResult PreflightChecker::checkObsNativeStreamDelay()
{
	config_t *config = obs_frontend_get_profile_config();
	if (!config) {
		return PreflightResult{true, PreflightFailureCode::None, {}};
	}

	const bool delayEnabled =
		config_get_bool(config, "Output", "DelayEnable");
	const int profileDelaySec = config_get_int(config, "Output", "DelaySec");

	int effectiveDelaySec = 0;
	if (delayEnabled) {
		effectiveDelaySec = profileDelaySec;
	}

	obs_output_t *output = obs_frontend_get_streaming_output();
	if (output) {
		const uint32_t outputDelaySec = obs_output_get_delay(output);
		if (outputDelaySec > 0) {
			effectiveDelaySec =
				static_cast<int>(outputDelaySec);
		}
		const uint32_t activeDelaySec =
			obs_output_get_active_delay(output);
		if (activeDelaySec > 0) {
			effectiveDelaySec =
				static_cast<int>(activeDelaySec);
		}
	}

	if (delayEnabled || effectiveDelaySec > 0) {
		const int reportedSeconds =
			effectiveDelaySec > 0 ? effectiveDelaySec : profileDelaySec;
		return fail(PreflightFailureCode::ObsNativeDelayEnabled,
			    QString::number(reportedSeconds > 0 ? reportedSeconds
								: 0));
	}

	return PreflightResult{true, PreflightFailureCode::None, {}};
}

PreflightResult PreflightChecker::run(RelayProcessState processState,
				      bool managedRelay,
				      const QString &apiBaseUrl,
				      const QString &sessionToken)
{
	Q_UNUSED(sessionToken);

	if (managedRelay && processState != RelayProcessState::Running &&
	    processState != RelayProcessState::Unmanaged) {
		return fail(PreflightFailureCode::RelayProcessNotRunning,
			    QStringLiteral("relay process is not running"));
	}

	QString healthDetail;
	if (!fetchRelayHealthy(apiBaseUrl, &healthDetail)) {
		if (healthDetail.contains(QStringLiteral("timed out"),
					  Qt::CaseInsensitive) ||
		    healthDetail.contains(QStringLiteral("refused"),
					  Qt::CaseInsensitive) ||
		    healthDetail.contains(QStringLiteral("closed"),
					  Qt::CaseInsensitive)) {
			return fail(PreflightFailureCode::RelayUnreachable,
				    healthDetail);
		}
		return fail(PreflightFailureCode::RelayUnhealthy, healthDetail);
	}

	QString ingestHost;
	quint16 ingestPort = 0;
	if (!ingestEndpoint(&ingestHost, &ingestPort)) {
		return fail(PreflightFailureCode::IngestNotListening,
			    QStringLiteral("DELAYDECK_INGEST_LISTEN is invalid"));
	}

	if (!probeTcpEndpoint(ingestHost, ingestPort, kIngestProbeTimeoutMs)) {
		return fail(PreflightFailureCode::IngestNotListening,
			    QStringLiteral("%1:%2 is not accepting connections")
				    .arg(ingestHost)
				    .arg(ingestPort));
	}

	if (delayDeckModeEnabled()) {
		const PreflightResult nativeDelay = checkObsNativeStreamDelay();
		if (!nativeDelay.ok) {
			return nativeDelay;
		}

		const PreflightResult destination =
			checkObsDestination(ingestHost, ingestPort);
		if (!destination.ok) {
			return destination;
		}
	}

	return PreflightResult{true, PreflightFailureCode::None, {}};
}
