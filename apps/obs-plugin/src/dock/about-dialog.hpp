#pragma once

#include <QDialog>

#include "util/github-release-checker.hpp"

class QLabel;

namespace delaydeck {

class AboutDialog final : public QDialog {
	Q_OBJECT

public:
	explicit AboutDialog(QWidget *parent = nullptr);

private:
	void startUpdateCheck();
	void applyReleaseCheckResult(const GithubReleaseCheckResult &result);
	void relayout();

	QLabel *update_status_ = nullptr;
};

} // namespace delaydeck
