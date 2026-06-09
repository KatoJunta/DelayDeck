#include "config/dock-settings.hpp"

#include "dock/delaydeck-dock.hpp"

#include <obs-frontend-api.h>

namespace delaydeck {

namespace {

DelayDeckDock *registered_dock_ = nullptr;

void dockSettingsCallback(obs_data_t *save_data, bool saving, void *)
{
	if (!registered_dock_) {
		return;
	}

	if (saving) {
		registered_dock_->saveSettings(save_data);
		return;
	}

	registered_dock_->loadSettings(save_data);
}

} // namespace

void registerDockSettingsCallbacks(DelayDeckDock *dock)
{
	registered_dock_ = dock;
	obs_frontend_add_save_callback(dockSettingsCallback, nullptr);
}

void unregisterDockSettingsCallbacks()
{
	obs_frontend_remove_save_callback(dockSettingsCallback, nullptr);
	registered_dock_ = nullptr;
}

} // namespace delaydeck
