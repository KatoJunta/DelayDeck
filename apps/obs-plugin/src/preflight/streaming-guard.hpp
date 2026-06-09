#pragma once

#include <obs-frontend-api.h>

class DelayDeckDock;

class StreamingGuard final {
public:
	static void install(DelayDeckDock *dock);
	static void uninstall();

private:
	static void onFrontendEvent(enum obs_frontend_event event, void *data);
	static void blockActiveStream(DelayDeckDock *dock);

	static DelayDeckDock *dock_;
	static bool block_next_stream_;
};
