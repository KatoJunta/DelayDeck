#pragma once

#include <QObject>
#include <QTcpSocket>
#include <QUrl>

class RelayEventSocket final : public QObject {
	Q_OBJECT

public:
	explicit RelayEventSocket(QObject *parent = nullptr);

	void open(const QUrl &url);
	void closeSocket();

signals:
	void connected();
	void disconnected();
	void textMessageReceived(const QString &message);
	void errorOccurred(const QString &detail);

private:
	void sendHandshake();
	void onReadyRead();
	bool parseFrames();
	bool parseServerHandshake(const QByteArray &data);

	QTcpSocket socket_;
	QUrl url_;
	QByteArray read_buffer_;
	bool handshake_complete_ = false;
};
