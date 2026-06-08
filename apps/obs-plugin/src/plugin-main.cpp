#include <obs-frontend-api.h>
#include <obs-module.h>

#include <QMainWindow>

#include "dock/delaydeck-dock.hpp"
#include "version.hpp"

OBS_DECLARE_MODULE()
OBS_MODULE_USE_DEFAULT_LOCALE("delaydeck", "ja-JP")

namespace {

constexpr const char *kDockId = "DelayDeckDock";
DelayDeckDock *g_dock = nullptr;

} // namespace

bool obs_module_load(void)
{
	obs_frontend_push_ui_translation(obs_module_get_string);

	const auto main_window =
		static_cast<QMainWindow *>(obs_frontend_get_main_window());
	g_dock = new DelayDeckDock(main_window);
	const bool added = obs_frontend_add_dock_by_id(
		kDockId, obs_module_text("DelayDeck"), g_dock);

	obs_frontend_pop_ui_translation();

	if (!added) {
		delete g_dock;
		g_dock = nullptr;
		blog(LOG_ERROR, "[DelayDeck] failed to register dock");
		return false;
	}

	blog(LOG_INFO, "[DelayDeck] loaded version %s", DELAYDECK_PLUGIN_VERSION);
	return true;
}

void obs_module_unload(void)
{
	if (g_dock) {
		// obs_frontend_remove_dock deletes the dock widget (via OBSDock).
		// Do not delete g_dock here — that would double-free and crash OBS.
		g_dock->shutdown();
		obs_frontend_remove_dock(kDockId);
		g_dock = nullptr;
	}

	blog(LOG_INFO, "[DelayDeck] unloaded");
}
