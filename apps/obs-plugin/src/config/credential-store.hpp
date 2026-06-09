#pragma once

#include <QString>

namespace delaydeck {

constexpr const char *kStreamKeyCredentialTarget = "DelayDeck.RelayOutputStreamKey";

bool writeStreamKey(const QString &streamKey);
bool readStreamKey(QString *streamKey);
void deleteStreamKey();

} // namespace delaydeck
