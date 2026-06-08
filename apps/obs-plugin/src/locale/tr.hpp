#pragma once

#include <obs-module.h>

#include <QString>

inline QString tr(const char *lookup)
{
	return QString::fromUtf8(obs_module_text(lookup));
}
