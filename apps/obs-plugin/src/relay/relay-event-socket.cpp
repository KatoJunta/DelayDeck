#include "relay-event-socket.hpp"

#include <QByteArray>
#include <QRandomGenerator>
#include <QUrlQuery>

namespace {

QByteArray makeWebSocketKey()
{
	auto bytes = QByteArray(16, '\0');
	for (int i = 0; i < 16; ++i) {
		bytes[i] = static_cast<char>(
			QRandomGenerator::global()->bounded(256));
	}
	return bytes.toBase64();
}

QString framePayloadToText(const QByteArray &payload)
{
	return QString::fromUtf8(payload);
}

} // namespace

RelayEventSocket::RelayEventSocket(QObject *parent) : QObject(parent)
{
	connect(&socket_, &QTcpSocket::connected, this,
		&RelayEventSocket::sendHandshake);
	connect(&socket_, &QTcpSocket::readyRead, this,
		&RelayEventSocket::onReadyRead);
	connect(&socket_, &QTcpSocket::disconnected, this, [this]() {
		handshake_complete_ = false;
		read_buffer_.clear();
		emit disconnected();
	});
	connect(&socket_, &QTcpSocket::errorOccurred, this,
		[this](QAbstractSocket::SocketError) {
			emit errorOccurred(socket_.errorString());
		});
}

void RelayEventSocket::open(const QUrl &url)
{
	if (socket_.state() != QAbstractSocket::UnconnectedState) {
		socket_.abort();
	}

	url_ = url;
	handshake_complete_ = false;
	read_buffer_.clear();

	const auto port =
		url.port(url.scheme() == QStringLiteral("wss") ? 443 : 80);
	socket_.connectToHost(url.host(), static_cast<quint16>(port));
}

void RelayEventSocket::closeSocket()
{
	if (socket_.state() != QAbstractSocket::UnconnectedState) {
		socket_.close();
	}
	handshake_complete_ = false;
	read_buffer_.clear();
}

void RelayEventSocket::sendHandshake()
{
	const auto key = makeWebSocketKey();

	const QString path =
		url_.path(QUrl::FullyEncoded) + url_.query(QUrl::FullyEncoded);

	QByteArray request;
	request += "GET " + path.toUtf8() + " HTTP/1.1\r\n";
	request += "Host: " + url_.host().toUtf8();
	if (url_.port() > 0) {
		request += ":" + QByteArray::number(url_.port());
	}
	request += "\r\n";
	request += "Upgrade: websocket\r\n";
	request += "Connection: Upgrade\r\n";
	request += "Sec-WebSocket-Key: " + key + "\r\n";
	request += "Sec-WebSocket-Version: 13\r\n";
	request += "\r\n";

	socket_.write(request);
}

void RelayEventSocket::onReadyRead()
{
	read_buffer_.append(socket_.readAll());

	if (!handshake_complete_) {
		const int header_end = read_buffer_.indexOf("\r\n\r\n");
		if (header_end < 0) {
			return;
		}

		const QByteArray header = read_buffer_.left(header_end + 4);
		if (!parseServerHandshake(header)) {
			closeSocket();
			emit errorOccurred(QStringLiteral("websocket handshake failed"));
			return;
		}

		handshake_complete_ = true;
		read_buffer_.remove(0, header_end + 4);
		emit connected();
	}

	while (parseFrames()) {
	}
}

bool RelayEventSocket::parseServerHandshake(const QByteArray &data)
{
	const auto header = QString::fromUtf8(data);
	return header.startsWith(QStringLiteral("HTTP/1.1 101")) &&
	       header.contains(QStringLiteral("Upgrade: websocket"),
			       Qt::CaseInsensitive);
}

bool RelayEventSocket::parseFrames()
{
	if (read_buffer_.size() < 2) {
		return false;
	}

	const auto byte0 = static_cast<unsigned char>(read_buffer_[0]);
	const auto byte1 = static_cast<unsigned char>(read_buffer_[1]);

	const bool masked = (byte1 & 0x80) != 0;
	quint64 payload_len = byte1 & 0x7F;
	int header_len = 2;

	if (payload_len == 126) {
		if (read_buffer_.size() < 4) {
			return false;
		}
		const auto high = static_cast<unsigned char>(read_buffer_[2]);
		const auto low = static_cast<unsigned char>(read_buffer_[3]);
		payload_len = (static_cast<quint64>(high) << 8) | low;
		header_len = 4;
	} else if (payload_len == 127) {
		if (read_buffer_.size() < 10) {
			return false;
		}
		payload_len = 0;
		for (int i = 0; i < 8; ++i) {
			payload_len = (payload_len << 8) |
				      static_cast<unsigned char>(
					      read_buffer_[2 + i]);
		}
		header_len = 10;
	}

	if (masked) {
		header_len += 4;
	}

	if (read_buffer_.size() < header_len + static_cast<int>(payload_len)) {
		return false;
	}

	QByteArray payload = read_buffer_.mid(header_len,
					      static_cast<int>(payload_len));
	if (masked) {
		const char mask[4] = {
			read_buffer_[header_len - 4],
			read_buffer_[header_len - 3],
			read_buffer_[header_len - 2],
			read_buffer_[header_len - 1],
		};
		for (int i = 0; i < payload.size(); ++i) {
			payload[i] = payload[i] ^ mask[i % 4];
		}
	}

	const int opcode = byte0 & 0x0F;
	if (opcode == 0x1) {
		emit textMessageReceived(framePayloadToText(payload));
	} else if (opcode == 0x8) {
		closeSocket();
		return false;
	}

	read_buffer_.remove(0, header_len + static_cast<int>(payload_len));
	return !read_buffer_.isEmpty();
}
