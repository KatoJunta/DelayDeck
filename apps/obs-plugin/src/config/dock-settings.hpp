#pragma once

#include <obs.h>

class DelayDeckDock;

namespace delaydeck {

constexpr const char *kDockSettingsKey = "delaydeck-dock";

void registerDockSettingsCallbacks(DelayDeckDock *dock);
void unregisterDockSettingsCallbacks();

} // namespace delaydeck
