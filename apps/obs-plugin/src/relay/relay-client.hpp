#pragma once

#include "relay-types.hpp"

#include <QObject>
#include <QTimer>
#include <QUrl>

class QNetworkAccessManager;
class QNetworkReply;
class RelayEventSocket;

enum class RelayLinkState {
	Disconnected,
	Unhealthy,
	AuthFailed,
	NoToken,
	Connected,
};

class RelayClient final : public QObject {
	Q_OBJECT

public:
	explicit RelayClient(QObject *parent = nullptr);
	~RelayClient() override;

	void start();
	void stop();

	RelayLinkState linkState() const { return link_state_; }
	QString apiBaseUrl() const { return api_base_url_; }

	void enableDelay(int targetDelaySeconds);
	void returnLive();
	void dumpBuffer();

signals:
	void linkStateChanged(RelayLinkState state);
	void healthUpdated(const RelayHealth &health);
	void statusUpdated(const RelayStatus &status);
	void controlFailed(const QString &code, const QString &message);
	void requestFailed(const QString &operation, const QString &detail);

private:
	void pollHealth();
	void fetchStatus();
	void connectEvents();
	void disconnectEvents();
	void scheduleWebSocketReconnect();

	void postControl(const QString &path, const QByteArray &body = {});

	void handleHealthReply(QNetworkReply *reply);
	void handleStatusReply(QNetworkReply *reply);
	void handleControlReply(QNetworkReply *reply, const QString &operation);
	void handleWebSocketMessage(const QString &message);

	void setLinkState(RelayLinkState state);
	static QUrl webSocketUrl(const QUrl &httpBase, const QString &token);

	QNetworkAccessManager *nam_;
	RelayEventSocket *event_socket_;
	QTimer poll_timer_;
	QTimer ws_reconnect_timer_;

	QString api_base_url_;
	QString session_token_;
	RelayLinkState link_state_ = RelayLinkState::Disconnected;
	bool running_ = false;
};
