#include "relay-process.hpp"

#include <QDir>
#include <QFileInfo>
#include <QProcessEnvironment>
#include <QRandomGenerator>
#include <QUrl>

#ifdef _WIN32
#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>
#endif

namespace {

constexpr int kDestructorKillWaitMs = 1000;
constexpr int kTerminateFallbackMs = 5000;

QString envOrDefault(const char *name, const QString &fallback)
{
	const auto value =
		QProcessEnvironment::systemEnvironment().value(QString::fromUtf8(name));
	return value.isEmpty() ? fallback : value;
}

bool managedRelayEnabled()
{
	const auto value = envOrDefault("DELAYDECK_MANAGED_RELAY", QString());
	if (value.isEmpty()) {
		return true;
	}
	const auto lower = value.toLower();
	return lower != QStringLiteral("0") && lower != QStringLiteral("false") &&
	       lower != QStringLiteral("no");
}

} // namespace

RelayProcess::RelayProcess(QObject *parent) : QObject(parent)
{
	managed_ = managedRelayEnabled();
	api_base_url_ = envOrDefault("DELAYDECK_RELAY_URL",
				     QStringLiteral("http://127.0.0.1:9400"));
	listen_address_ = listenAddressFromRelayUrl(api_base_url_);

	process_.setProcessChannelMode(QProcess::MergedChannels);

	connect(&process_, &QProcess::started, this, &RelayProcess::onProcessStarted);
	connect(&process_, &QProcess::errorOccurred, this,
		&RelayProcess::onProcessError);
	connect(&process_,
		QOverload<int, QProcess::ExitStatus>::of(&QProcess::finished), this,
		&RelayProcess::onProcessFinished);

	terminate_fallback_timer_.setSingleShot(true);
	terminate_fallback_timer_.setInterval(kTerminateFallbackMs);
	connect(&terminate_fallback_timer_, &QTimer::timeout, this, [this]() {
		if (intentional_stop_ && process_.state() != QProcess::NotRunning) {
			process_.kill();
		}
	});

	if (!managed_) {
		setState(RelayProcessState::Unmanaged);
	}
}

RelayProcess::~RelayProcess()
{
	shutdown();
}

void RelayProcess::shutdown()
{
	pending_restart_ = false;
	intentional_stop_ = true;
	terminate_fallback_timer_.stop();

	if (process_.state() != QProcess::NotRunning) {
		process_.kill();
		process_.waitForFinished(kDestructorKillWaitMs);
	}

	intentional_stop_ = false;
}

qint64 RelayProcess::processId() const
{
	if (process_.state() == QProcess::NotRunning) {
		return 0;
	}
	return process_.processId();
}

void RelayProcess::startRelay()
{
	if (!managed_) {
		return;
	}
	if (process_.state() != QProcess::NotRunning) {
		return;
	}

	relay_binary_ = resolveRelayBinary();
	if (relay_binary_.isEmpty() || !QFileInfo::exists(relay_binary_)) {
		last_error_ = QStringLiteral("delaydeck-relay binary not found");
		setState(RelayProcessState::FailedToStart);
		return;
	}

	session_token_ = generateSessionToken();
	last_error_.clear();
	setState(RelayProcessState::Starting);
	emit credentialsReady(api_base_url_, session_token_);

	process_.setProgram(relay_binary_);
	process_.setArguments({QStringLiteral("--listen"), listen_address_,
			       QStringLiteral("--mock-auto-connect")});

	QProcessEnvironment env = QProcessEnvironment::systemEnvironment();
	env.insert(QStringLiteral("DELAYDECK_SESSION_TOKEN"), session_token_);
	process_.setProcessEnvironment(env);
	process_.start();
}

void RelayProcess::stopRelay()
{
	if (!managed_) {
		return;
	}
	if (process_.state() == QProcess::NotRunning) {
		completeIntentionalStop();
		return;
	}

	intentional_stop_ = true;
	setState(RelayProcessState::Stopping);
	terminate_fallback_timer_.start();
	process_.terminate();
}

void RelayProcess::restartRelay()
{
	if (!managed_) {
		return;
	}

	pending_restart_ = true;
	if (process_.state() == QProcess::NotRunning) {
		completeIntentionalStop();
		return;
	}

	stopRelay();
}

void RelayProcess::completeIntentionalStop()
{
	if (!intentional_stop_ && !pending_restart_) {
		return;
	}

	intentional_stop_ = false;
	setState(RelayProcessState::Idle);

	if (pending_restart_) {
		pending_restart_ = false;
		startRelay();
	}
}

void RelayProcess::setState(RelayProcessState state)
{
	if (state_ == state) {
		return;
	}
	state_ = state;
	emit stateChanged(state);
}

void RelayProcess::onProcessStarted()
{
	setState(RelayProcessState::Running);
}

void RelayProcess::onProcessFinished(int exitCode, QProcess::ExitStatus status)
{
	terminate_fallback_timer_.stop();

	if (intentional_stop_) {
		completeIntentionalStop();
		return;
	}

	emit processDied();

	const QByteArray output = process_.readAll();
	if (!output.isEmpty()) {
		const QString text = QString::fromUtf8(output).trimmed();
		const int lineLimit = 240;
		last_error_ = text.length() > lineLimit
				      ? text.left(lineLimit) + QStringLiteral("…")
				      : text;
	}

	if (status == QProcess::CrashExit) {
		if (last_error_.isEmpty()) {
			last_error_ = QStringLiteral("relay process crashed");
		}
		setState(RelayProcessState::Crashed);
		return;
	}

	if (exitCode != 0) {
		if (last_error_.isEmpty()) {
			last_error_ = QStringLiteral("relay exited with code %1")
					      .arg(exitCode);
		}
		setState(RelayProcessState::Crashed);
		return;
	}

	if (state_ == RelayProcessState::Running ||
	    state_ == RelayProcessState::Starting) {
		if (last_error_.isEmpty()) {
			last_error_ = QStringLiteral("relay process exited");
		}
		setState(RelayProcessState::Crashed);
	}
}

void RelayProcess::onProcessError(QProcess::ProcessError error)
{
	if (intentional_stop_) {
		return;
	}
	if (error == QProcess::FailedToStart) {
		last_error_ = process_.errorString();
		setState(RelayProcessState::FailedToStart);
	}
}

QString RelayProcess::resolveRelayBinary() const
{
	const auto fromEnv = envOrDefault("DELAYDECK_RELAY_BIN", QString());
	if (!fromEnv.isEmpty()) {
		return QFileInfo(fromEnv).absoluteFilePath();
	}

	const QString pluginDir = pluginModuleDirectory();
	QStringList candidates;
	if (!pluginDir.isEmpty()) {
		const QDir dir(pluginDir);
		candidates << dir.absoluteFilePath(QStringLiteral("delaydeck-relay.exe"));
		candidates << dir.absoluteFilePath(QStringLiteral("delaydeck-relay"));
		candidates << dir.absoluteFilePath(
			QStringLiteral("../delaydeck/delaydeck-relay.exe"));
		candidates << dir.absoluteFilePath(
			QStringLiteral("../delaydeck/delaydeck-relay"));
	}

	for (const auto &path : candidates) {
		if (QFileInfo::exists(path)) {
			return QFileInfo(path).absoluteFilePath();
		}
	}

	return {};
}

QString RelayProcess::generateSessionToken()
{
	QByteArray bytes(32, Qt::Uninitialized);
	auto *rng = QRandomGenerator::system();
	for (int i = 0; i < bytes.size(); ++i) {
		bytes[i] = static_cast<char>(rng->bounded(256));
	}
	return QString::fromLatin1(bytes.toHex());
}

QString RelayProcess::listenAddressFromRelayUrl(const QString &relayUrl)
{
	const QUrl url(relayUrl);
	if (!url.isValid() || url.host().isEmpty()) {
		return QStringLiteral("127.0.0.1:9400");
	}

	const int port = url.port(9400);
	return QStringLiteral("%1:%2").arg(url.host(), QString::number(port));
}

QString RelayProcess::pluginModuleDirectory()
{
#ifdef _WIN32
	HMODULE module = nullptr;
	const auto self = reinterpret_cast<LPCWSTR>(&pluginModuleDirectory);
	if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
					GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
				self, &module)) {
		return {};
	}

	wchar_t path[MAX_PATH];
	const DWORD length = GetModuleFileNameW(module, path, MAX_PATH);
	if (length == 0 || length >= MAX_PATH) {
		return {};
	}

	return QFileInfo(QString::fromWCharArray(path, static_cast<int>(length)))
		.absolutePath();
#else
	return {};
#endif
}
