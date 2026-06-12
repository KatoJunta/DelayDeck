#pragma once

#include <QObject>
#include <QProcess>
#include <QString>
#include <QTimer>

enum class RelayProcessState {
	Unmanaged,
	Idle,
	Starting,
	Running,
	Stopping,
	Crashed,
	FailedToStart,
};

class RelayProcess final : public QObject {
	Q_OBJECT

public:
	explicit RelayProcess(QObject *parent = nullptr);
	~RelayProcess() override;

	bool isManaged() const { return managed_; }
	RelayProcessState state() const { return state_; }
	qint64 processId() const;
	QString sessionToken() const { return session_token_; }
	QString apiBaseUrl() const { return api_base_url_; }
	QString relayBinary() const { return relay_binary_; }
	QString lastError() const { return last_error_; }

	void startRelay();
	void stopRelay();
	void restartRelay();
	void shutdown();

signals:
	void stateChanged(RelayProcessState state);
	void credentialsReady(const QString &apiBaseUrl, const QString &sessionToken);
	void processDied();

private:
	void setState(RelayProcessState state);
	void onProcessStarted();
	void onProcessFinished(int exitCode, QProcess::ExitStatus status);
	void onProcessError(QProcess::ProcessError error);
	void completeIntentionalStop();

	QString resolveRelayBinary() const;
	static QString generateSessionToken();
	static QString listenAddressFromRelayUrl(const QString &relayUrl);

	bool managed_ = true;
	bool intentional_stop_ = false;
	bool pending_restart_ = false;
	RelayProcessState state_ = RelayProcessState::Idle;

	QString api_base_url_;
	QString session_token_;
	QString relay_binary_;
	QString listen_address_;
	QString last_error_;

	QProcess process_;
	QTimer terminate_fallback_timer_;
};
