#pragma once

#include <QString>

namespace delaydeck {

bool isValidRelayOutputUrl(const QString &rawUrl, QString *normalizedUrl = nullptr);

} // namespace delaydeck
