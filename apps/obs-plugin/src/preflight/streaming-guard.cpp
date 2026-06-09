#include "streaming-guard.hpp"

#include "dock/delaydeck-dock.hpp"
#include "preflight/preflight-dialog.hpp"

#include <QTimer>

DelayDeckDock *StreamingGuard::dock_ = nullptr;
bool StreamingGuard::block_next_stream_ = false;

void StreamingGuard::install(DelayDeckDock *dock)
{
	dock_ = dock;
	obs_frontend_add_event_callback(onFrontendEvent, dock);
}

void StreamingGuard::uninstall()
{
	if (dock_) {
		obs_frontend_remove_event_callback(onFrontendEvent, dock_);
		dock_ = nullptr;
	}
	block_next_stream_ = false;
}

void StreamingGuard::blockActiveStream(DelayDeckDock *dock)
{
	Q_UNUSED(dock);
	if (obs_frontend_streaming_active()) {
		obs_frontend_streaming_stop();
	}
}

void StreamingGuard::onFrontendEvent(enum obs_frontend_event event, void *data)
{
	auto *dock = static_cast<DelayDeckDock *>(data);
	if (!dock) {
		return;
	}

	dock->handleFrontendEvent(event);

	if (event == OBS_FRONTEND_EVENT_STREAMING_STARTING) {
		const PreflightResult result = dock->runPreflight();
		dock->setLastPreflightResult(result);
		block_next_stream_ = !result.ok;

		if (!result.ok) {
			QTimer::singleShot(0, dock, [dock]() {
				PreflightDialog::showFailure(
					dock, dock->lastPreflightResult());
			});
		}
		return;
	}

	if (event == OBS_FRONTEND_EVENT_STREAMING_STARTED && block_next_stream_) {
		block_next_stream_ = false;
		QTimer::singleShot(0, dock,
				    [dock]() { blockActiveStream(dock); });
	}
}
