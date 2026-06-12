#pragma once

#include <QString>

namespace delaydeck {

struct GithubReleaseCheckResult {
	bool success = false;
	QString tagName;
	QString htmlUrl;
};

GithubReleaseCheckResult fetchLatestGithubRelease();

} // namespace delaydeck
