#include "obs/obs-install-path.hpp"

#include <QDir>
#include <QFileInfo>

#ifdef _WIN32
#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>
#endif

namespace delaydeck {

QString pluginModuleDirectory()
{
#ifdef _WIN32
	HMODULE module = nullptr;
	const auto self = reinterpret_cast<LPCWSTR>(&pluginModuleDirectory);
	if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
					GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
				self, &module)) {
		return {};
	}

	wchar_t path[MAX_PATH];
	const DWORD length = GetModuleFileNameW(module, path, MAX_PATH);
	if (length == 0 || length >= MAX_PATH) {
		return {};
	}

	return QFileInfo(QString::fromWCharArray(path, static_cast<int>(length)))
		.absolutePath();
#else
	return {};
#endif
}

QString obsInstallDirectory()
{
	const QString pluginDir = pluginModuleDirectory();
	if (pluginDir.isEmpty()) {
		return {};
	}

	QDir dir(pluginDir);
	if (!dir.cdUp()) {
		return {};
	}
	if (!dir.cdUp()) {
		return {};
	}

	return dir.absolutePath();
}

} // namespace delaydeck
