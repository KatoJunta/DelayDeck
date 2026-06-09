#pragma once

#include <obs-module.h>

#include <QString>

namespace delaydeck {

// QWidget subclasses inherit QObject::tr(); use this helper instead.
inline QString tr(const char *lookup)
{
	return QString::fromUtf8(obs_module_text(lookup));
}

} // namespace delaydeck
