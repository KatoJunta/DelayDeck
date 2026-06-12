#include "util/github-release-checker.hpp"

#include <QJsonDocument>
#include <QJsonObject>
#include <QUrl>

#ifdef _WIN32
#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>
#include <winhttp.h>
#endif

namespace delaydeck {

namespace {

constexpr const char *kLatestReleaseApiUrl =
	"https://api.github.com/repos/KatoJunta/DelayDeck/releases/latest";
constexpr int kRequestTimeoutMs = 10000;

#ifdef _WIN32

bool queryHttpStatus(HINTERNET request, DWORD *status)
{
	DWORD statusCode = 0;
	DWORD statusSize = sizeof(statusCode);
	if (!WinHttpQueryHeaders(request,
				 WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
				 WINHTTP_HEADER_NAME_BY_INDEX, &statusCode, &statusSize,
				 WINHTTP_NO_HEADER_INDEX)) {
		return false;
	}
	*status = statusCode;
	return true;
}

QString readResponseBody(HINTERNET request)
{
	QByteArray body;
	for (;;) {
		DWORD available = 0;
		if (!WinHttpQueryDataAvailable(request, &available)) {
			return {};
		}
		if (available == 0) {
			break;
		}

		QByteArray chunk(static_cast<int>(available), Qt::Uninitialized);
		DWORD read = 0;
		if (!WinHttpReadData(request, chunk.data(), available, &read)) {
			return {};
		}
		chunk.resize(static_cast<int>(read));
		body.append(chunk);
	}
	return QString::fromUtf8(body);
}

GithubReleaseCheckResult fetchLatestGithubReleaseWin()
{
	GithubReleaseCheckResult result;

	const QUrl url(QString::fromUtf8(kLatestReleaseApiUrl));
	if (!url.isValid() || url.scheme() != QStringLiteral("https") ||
	    url.host().isEmpty()) {
		return result;
	}

	QString requestPath = url.path(QUrl::FullyEncoded);
	if (!url.query().isEmpty()) {
		requestPath += QLatin1Char('?') + url.query(QUrl::FullyEncoded);
	}

	const INTERNET_PORT port = static_cast<INTERNET_PORT>(url.port(443));

	HINTERNET session = WinHttpOpen(L"DelayDeck-OBS-Plugin",
					WINHTTP_ACCESS_TYPE_DEFAULT_PROXY,
					WINHTTP_NO_PROXY_NAME,
					WINHTTP_NO_PROXY_BYPASS, 0);
	if (!session) {
		return result;
	}

	WinHttpSetTimeouts(session, kRequestTimeoutMs, kRequestTimeoutMs,
			   kRequestTimeoutMs, kRequestTimeoutMs);

	const QString host = url.host();
	HINTERNET connection =
		WinHttpConnect(session, host.toStdWString().c_str(), port, 0);
	if (!connection) {
		WinHttpCloseHandle(session);
		return result;
	}

	HINTERNET request = WinHttpOpenRequest(
		connection, L"GET", requestPath.toStdWString().c_str(), nullptr,
		WINHTTP_NO_REFERER, WINHTTP_DEFAULT_ACCEPT_TYPES, WINHTTP_FLAG_SECURE);
	if (!request) {
		WinHttpCloseHandle(connection);
		WinHttpCloseHandle(session);
		return result;
	}

	const wchar_t *headers = L"User-Agent: DelayDeck-OBS-Plugin\r\n";
	if (!WinHttpAddRequestHeaders(request, headers, static_cast<DWORD>(-1),
				      WINHTTP_ADDREQ_FLAG_ADD)) {
		WinHttpCloseHandle(request);
		WinHttpCloseHandle(connection);
		WinHttpCloseHandle(session);
		return result;
	}

	if (!WinHttpSendRequest(request, WINHTTP_NO_ADDITIONAL_HEADERS, 0,
				WINHTTP_NO_REQUEST_DATA, 0, 0, 0) ||
	    !WinHttpReceiveResponse(request, nullptr)) {
		WinHttpCloseHandle(request);
		WinHttpCloseHandle(connection);
		WinHttpCloseHandle(session);
		return result;
	}

	DWORD statusCode = 0;
	if (!queryHttpStatus(request, &statusCode) || statusCode != 200) {
		WinHttpCloseHandle(request);
		WinHttpCloseHandle(connection);
		WinHttpCloseHandle(session);
		return result;
	}

	const QString body = readResponseBody(request);
	WinHttpCloseHandle(request);
	WinHttpCloseHandle(connection);
	WinHttpCloseHandle(session);

	if (body.isEmpty()) {
		return result;
	}

	const QJsonDocument document = QJsonDocument::fromJson(body.toUtf8());
	if (!document.isObject()) {
		return result;
	}

	const QJsonObject release = document.object();
	result.tagName = release.value(QStringLiteral("tag_name")).toString();
	result.htmlUrl = release.value(QStringLiteral("html_url")).toString();
	result.success = !result.tagName.isEmpty() && !result.htmlUrl.isEmpty();
	return result;
}

#endif

} // namespace

GithubReleaseCheckResult fetchLatestGithubRelease()
{
#ifdef _WIN32
	return fetchLatestGithubReleaseWin();
#else
	return {};
#endif
}

} // namespace delaydeck
