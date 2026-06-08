#include "relay-client.hpp"

#include "relay-event-socket.hpp"

#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QProcessEnvironment>
#include <QUrlQuery>

namespace {

constexpr int kPollIntervalMs = 3000;
constexpr int kWebSocketReconnectMs = 2000;

QString envOrDefault(const char *name, const QString &fallback)
{
	const auto value =
		QProcessEnvironment::systemEnvironment().value(QString::fromUtf8(name));
	return value.isEmpty() ? fallback : value;
}

QNetworkRequest authorizedRequest(const QUrl &url, const QString &token)
{
	QNetworkRequest request(url);
	request.setHeader(QNetworkRequest::ContentTypeHeader,
			  QStringLiteral("application/json"));
	if (!token.isEmpty()) {
		request.setRawHeader("Authorization",
				     QByteArray("Bearer ") + token.toUtf8());
	}
	return request;
}

} // namespace

RelayClient::RelayClient(QObject *parent) : QObject(parent)
{
	nam_ = new QNetworkAccessManager(this);
	event_socket_ = new RelayEventSocket(this);

	api_base_url_ = envOrDefault("DELAYDECK_RELAY_URL",
				     QStringLiteral("http://127.0.0.1:9400"));
	session_token_ =
		envOrDefault("DELAYDECK_SESSION_TOKEN", QString());

	poll_timer_.setInterval(kPollIntervalMs);
	connect(&poll_timer_, &QTimer::timeout, this, &RelayClient::pollHealth);

	ws_reconnect_timer_.setSingleShot(true);
	ws_reconnect_timer_.setInterval(kWebSocketReconnectMs);
	connect(&ws_reconnect_timer_, &QTimer::timeout, this,
		&RelayClient::connectEvents);

	connect(event_socket_, &RelayEventSocket::connected, this, [this]() {
		ws_reconnect_timer_.stop();
	});
	connect(event_socket_, &RelayEventSocket::disconnected, this, [this]() {
		if (running_ && link_state_ == RelayLinkState::Connected) {
			scheduleWebSocketReconnect();
		}
	});
	connect(event_socket_, &RelayEventSocket::textMessageReceived, this,
		&RelayClient::handleWebSocketMessage);
}

RelayClient::~RelayClient()
{
	stop();
}

void RelayClient::start()
{
	if (running_) {
		return;
	}
	running_ = true;

	if (session_token_.isEmpty()) {
		setLinkState(RelayLinkState::NoToken);
	} else {
		setLinkState(RelayLinkState::Disconnected);
	}

	pollHealth();
	poll_timer_.start();
}

void RelayClient::stop()
{
	running_ = false;
	poll_timer_.stop();
	ws_reconnect_timer_.stop();
	disconnectEvents();
}

void RelayClient::enableDelay(int targetDelaySeconds)
{
	QJsonObject body;
	body.insert(QStringLiteral("target_delay_seconds"), targetDelaySeconds);
	const auto payload =
		QJsonDocument(body).toJson(QJsonDocument::Compact);
	postControl(QStringLiteral("/v1/control/enable-delay"), payload);
}

void RelayClient::returnLive()
{
	postControl(QStringLiteral("/v1/control/return-live"));
}

void RelayClient::dumpBuffer()
{
	postControl(QStringLiteral("/v1/control/dump-buffer"));
}

void RelayClient::pollHealth()
{
	if (!running_) {
		return;
	}

	if (session_token_.isEmpty()) {
		setLinkState(RelayLinkState::NoToken);
		return;
	}

	const QUrl url(api_base_url_ + QStringLiteral("/v1/health"));
	auto *reply = nam_->get(QNetworkRequest(url));
	connect(reply, &QNetworkReply::finished, this, [this, reply]() {
		handleHealthReply(reply);
		reply->deleteLater();
	});
}

void RelayClient::fetchStatus()
{
	const QUrl url(api_base_url_ + QStringLiteral("/v1/status"));
	auto *reply = nam_->get(authorizedRequest(url, session_token_));
	connect(reply, &QNetworkReply::finished, this, [this, reply]() {
		handleStatusReply(reply);
		reply->deleteLater();
	});
}

void RelayClient::connectEvents()
{
	if (!running_ || session_token_.isEmpty() ||
	    link_state_ != RelayLinkState::Connected) {
		return;
	}

	event_socket_->open(webSocketUrl(QUrl(api_base_url_), session_token_));
}

void RelayClient::disconnectEvents()
{
	ws_reconnect_timer_.stop();
	event_socket_->closeSocket();
}

void RelayClient::scheduleWebSocketReconnect()
{
	if (!ws_reconnect_timer_.isActive()) {
		ws_reconnect_timer_.start();
	}
}

void RelayClient::postControl(const QString &path, const QByteArray &body)
{
	if (link_state_ != RelayLinkState::Connected) {
		emit controlFailed(QStringLiteral("relay_unavailable"), QString());
		return;
	}

	const QUrl url(api_base_url_ + path);
	auto *reply = nam_->post(authorizedRequest(url, session_token_), body);
	connect(reply, &QNetworkReply::finished, this, [this, reply, path]() {
		handleControlReply(reply, path);
		reply->deleteLater();
	});
}

void RelayClient::handleHealthReply(QNetworkReply *reply)
{
	if (!running_) {
		return;
	}

	if (reply->error() != QNetworkReply::NoError) {
		disconnectEvents();
		setLinkState(RelayLinkState::Disconnected);
		emit requestFailed(QStringLiteral("health"),
				   reply->errorString());
		return;
	}

	const auto doc = QJsonDocument::fromJson(reply->readAll());
	if (!doc.isObject()) {
		disconnectEvents();
		setLinkState(RelayLinkState::Disconnected);
		emit requestFailed(QStringLiteral("health"),
				   QStringLiteral("invalid JSON response"));
		return;
	}

	const auto health = parseHealth(doc.object());
	emit healthUpdated(health);

	if (!health.healthy) {
		disconnectEvents();
		setLinkState(RelayLinkState::Unhealthy);
		return;
	}

	fetchStatus();
}

void RelayClient::handleStatusReply(QNetworkReply *reply)
{
	if (!running_) {
		return;
	}

	const int httpStatus =
		reply->attribute(QNetworkRequest::HttpStatusCodeAttribute)
			.toInt();
	const QByteArray body = reply->readAll();

	if (reply->error() != QNetworkReply::NoError && httpStatus == 0) {
		disconnectEvents();
		setLinkState(RelayLinkState::Disconnected);
		emit requestFailed(QStringLiteral("status"),
				   reply->errorString());
		return;
	}

	const auto doc = QJsonDocument::fromJson(body);
	if (!doc.isObject()) {
		disconnectEvents();
		setLinkState(RelayLinkState::Disconnected);
		emit requestFailed(QStringLiteral("status"),
				   QStringLiteral("invalid JSON response"));
		return;
	}

	if (httpStatus == 401) {
		disconnectEvents();
		setLinkState(RelayLinkState::AuthFailed);
		RelayControlError error;
		if (parseControlError(doc.object(), &error)) {
			emit controlFailed(error.code, error.message);
		}
		return;
	}

	if (httpStatus >= 400) {
		disconnectEvents();
		setLinkState(RelayLinkState::Disconnected);
		emit requestFailed(QStringLiteral("status"),
				   QStringLiteral("HTTP %1").arg(httpStatus));
		return;
	}

	const auto status = parseStatus(doc.object());
	emit statusUpdated(status);
	setLinkState(RelayLinkState::Connected);
	connectEvents();
}

void RelayClient::handleControlReply(QNetworkReply *reply,
				     const QString &operation)
{
	if (reply->error() != QNetworkReply::NoError) {
		emit requestFailed(operation, reply->errorString());
		return;
	}

	const auto doc = QJsonDocument::fromJson(reply->readAll());
	if (!doc.isObject()) {
		emit requestFailed(operation,
				   QStringLiteral("invalid JSON response"));
		return;
	}

	const int httpStatus =
		reply->attribute(QNetworkRequest::HttpStatusCodeAttribute)
			.toInt();

	if (httpStatus >= 400) {
		RelayControlError error;
		if (parseControlError(doc.object(), &error)) {
			emit controlFailed(error.code, error.message);
		} else {
			emit controlFailed(QStringLiteral("request_failed"),
					   QStringLiteral("HTTP %1").arg(httpStatus));
		}
		return;
	}

	const auto statusObj = doc.object().value(QStringLiteral("status"));
	if (statusObj.isObject()) {
		emit statusUpdated(parseStatus(statusObj.toObject()));
	}
}

void RelayClient::handleWebSocketMessage(const QString &message)
{
	const auto doc = QJsonDocument::fromJson(message.toUtf8());
	if (!doc.isObject()) {
		return;
	}

	const auto root = doc.object();
	if (root.value(QStringLiteral("type")).toString() !=
	    QStringLiteral("state.changed")) {
		return;
	}

	const auto payload = root.value(QStringLiteral("payload")).toObject();
	const auto statusObj = payload.value(QStringLiteral("status"));
	if (statusObj.isObject()) {
		emit statusUpdated(parseStatus(statusObj.toObject()));
	}
}

void RelayClient::setLinkState(RelayLinkState state)
{
	if (link_state_ == state) {
		return;
	}
	link_state_ = state;
	emit linkStateChanged(state);
}

QUrl RelayClient::webSocketUrl(const QUrl &httpBase, const QString &token)
{
	QUrl wsBase = httpBase;
	const auto scheme = wsBase.scheme();
	if (scheme == QStringLiteral("https")) {
		wsBase.setScheme(QStringLiteral("wss"));
	} else {
		wsBase.setScheme(QStringLiteral("ws"));
	}

	QUrl eventsUrl = wsBase;
	eventsUrl.setPath(QStringLiteral("/v1/events"));

	QUrlQuery query;
	query.addQueryItem(QStringLiteral("token"), token);
	eventsUrl.setQuery(query);
	return eventsUrl;
}
