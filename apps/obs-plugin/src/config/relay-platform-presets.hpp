#pragma once

#include <QString>
#include <QVector>

namespace delaydeck {

struct PlatformPreset {
	const char *localeKey;
	QString outputUrl;
};

QVector<PlatformPreset> platformPresets();
int platformPresetIndexForUrl(const QString &outputUrl);

} // namespace delaydeck
