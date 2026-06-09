#include "config/relay-destination-url.hpp"

#include <QRegularExpression>
#include <QUrl>

namespace delaydeck {

namespace {

bool hostFromManualParse(const QString &url, QString *host)
{
	static const QRegularExpression pattern(
		QStringLiteral("^(?:rtmp|rtmps)://([^/?#]+)"),
		QRegularExpression::CaseInsensitiveOption);
	const QRegularExpressionMatch match = pattern.match(url.trimmed());
	if (!match.hasMatch()) {
		return false;
	}

	*host = match.captured(1).trimmed();
	if (host->isEmpty()) {
		return false;
	}

	const int bracketEnd = host->indexOf(QLatin1Char(']'));
	if (host->startsWith(QLatin1Char('['))) {
		if (bracketEnd <= 0) {
			return false;
		}
		*host = host->mid(1, bracketEnd - 1);
	} else {
		const int colon = host->lastIndexOf(QLatin1Char(':'));
		if (colon > 0 && !host->contains(QLatin1Char('.'))) {
			*host = host->left(colon);
		}
	}

	return !host->isEmpty();
}

} // namespace

bool isValidRelayOutputUrl(const QString &rawUrl, QString *normalizedUrl)
{
	const QString trimmed = rawUrl.trimmed();
	if (trimmed.isEmpty()) {
		return false;
	}

	const QString lower = trimmed.toLower();
	if (!lower.startsWith(QStringLiteral("rtmp://")) &&
	    !lower.startsWith(QStringLiteral("rtmps://"))) {
		return false;
	}

	QString host;
	const QUrl parsed(trimmed);
	if (!parsed.host().isEmpty()) {
		host = parsed.host();
	} else if (!hostFromManualParse(trimmed, &host)) {
		return false;
	}

	if (normalizedUrl) {
		*normalizedUrl = trimmed;
	}

	return true;
}

} // namespace delaydeck
