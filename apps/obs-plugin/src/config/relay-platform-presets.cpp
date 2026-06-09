#include "config/relay-platform-presets.hpp"

namespace delaydeck {

namespace {

QString normalizeUrlKey(const QString &url)
{
	return url.trimmed().toLower();
}

} // namespace

QVector<PlatformPreset> platformPresets()
{
	return {
		{"Setup.Platform.Custom", QString()},
		{"Setup.Platform.Youtube",
		 QStringLiteral("rtmp://a.rtmp.youtube.com/live2")},
		{"Setup.Platform.Twitch",
		 QStringLiteral("rtmp://live.twitch.tv/app")},
	};
}

int platformPresetIndexForUrl(const QString &outputUrl)
{
	const QString key = normalizeUrlKey(outputUrl);
	if (key.isEmpty()) {
		return 0;
	}

	const QVector<PlatformPreset> presets = platformPresets();
	for (int i = 1; i < presets.size(); ++i) {
		if (normalizeUrlKey(presets[i].outputUrl) == key) {
			return i;
		}
	}

	return 0;
}

} // namespace delaydeck
