#include "dock/about-dialog.hpp"

#include "locale/tr.hpp"
#include "obs/obs-install-path.hpp"
#include "version.hpp"

#include <QApplication>
#include <QDesktopServices>
#include <QFont>
#include <QFrame>
#include <QHBoxLayout>
#include <QLabel>
#include <QPointer>
#include <QPushButton>
#include <QUrl>
#include <QVBoxLayout>

#include <thread>

namespace delaydeck {

namespace {

constexpr const char *kGithubUrl = "https://github.com/KatoJunta/DelayDeck";
constexpr const char *kXUrl = "https://x.com/KatoJunta";

QString linkHtml(const QString &text, const QString &url)
{
	return QStringLiteral("<a href=\"%1\">%2</a>")
		.arg(url.toHtmlEscaped(), text.toHtmlEscaped());
}

QFrame *makeSeparator(QWidget *parent)
{
	auto *line = new QFrame(parent);
	line->setFrameShape(QFrame::HLine);
	line->setFrameShadow(QFrame::Sunken);
	return line;
}

QString normalizeVersionTag(const QString &raw)
{
	QString tag = raw.trimmed();
	if (tag.startsWith(QLatin1Char('v'), Qt::CaseInsensitive)) {
		tag = tag.mid(1);
	}
	return tag;
}

bool parseVersionTriple(const QString &raw, int *major, int *minor, int *patch)
{
	const QStringList parts = raw.split(QLatin1Char('.'));
	if (parts.size() != 3) {
		return false;
	}

	bool okMajor = false;
	bool okMinor = false;
	bool okPatch = false;
	*major = parts[0].toInt(&okMajor);
	*minor = parts[1].toInt(&okMinor);
	*patch = parts[2].toInt(&okPatch);
	return okMajor && okMinor && okPatch;
}

bool isRemoteVersionNewer(const QString &remoteTag, const QString &localVersion)
{
	const QString remote = normalizeVersionTag(remoteTag);
	const QString local = normalizeVersionTag(localVersion);

	int remoteMajor = 0;
	int remoteMinor = 0;
	int remotePatch = 0;
	int localMajor = 0;
	int localMinor = 0;
	int localPatch = 0;
	if (!parseVersionTriple(remote, &remoteMajor, &remoteMinor, &remotePatch) ||
	    !parseVersionTriple(local, &localMajor, &localMinor, &localPatch)) {
		return false;
	}

	if (remoteMajor != localMajor) {
		return remoteMajor > localMajor;
	}
	if (remoteMinor != localMinor) {
		return remoteMinor > localMinor;
	}
	return remotePatch > localPatch;
}

} // namespace

AboutDialog::AboutDialog(QWidget *parent) : QDialog(parent)
{
	setWindowTitle(delaydeck::tr("About.Title"));
	setModal(true);

	auto *layout = new QVBoxLayout(this);
	layout->setContentsMargins(20, 20, 20, 16);
	layout->setSpacing(12);

	// Header: product name, version, and update status on one line each.
	auto *title = new QLabel(QStringLiteral("DelayDeck"), this);
	QFont titleFont = title->font();
	titleFont.setPointSizeF(titleFont.pointSizeF() * 1.5);
	titleFont.setBold(true);
	title->setFont(titleFont);
	title->setAlignment(Qt::AlignHCenter);
	layout->addWidget(title);

	auto *versionRow = new QHBoxLayout();
	versionRow->setSpacing(8);
	versionRow->addStretch();

	auto *version = new QLabel(
		delaydeck::tr("About.Version")
			.arg(QString::fromUtf8(DELAYDECK_PLUGIN_VERSION)),
		this);
	version->setTextInteractionFlags(Qt::TextSelectableByMouse);
	versionRow->addWidget(version);

	update_status_ = new QLabel(this);
	update_status_->setOpenExternalLinks(true);
	update_status_->setTextInteractionFlags(Qt::TextBrowserInteraction);
	update_status_->hide();
	versionRow->addWidget(update_status_);

	versionRow->addStretch();
	layout->addLayout(versionRow);

	auto *description = new QLabel(delaydeck::tr("About.Description"), this);
	description->setWordWrap(true);
	description->setAlignment(Qt::AlignHCenter);
	layout->addWidget(description);

	layout->addWidget(makeSeparator(this));

	// Links: one centered row instead of a stacked list.
	auto *linksRow = new QHBoxLayout();
	linksRow->setSpacing(16);
	linksRow->addStretch();

	auto *links = new QLabel(
		QStringLiteral("%1&nbsp;&nbsp;·&nbsp;&nbsp;%2")
			.arg(linkHtml(delaydeck::tr("About.GithubLink"),
				      QString::fromUtf8(kGithubUrl)),
			     linkHtml(delaydeck::tr("About.XLink"),
				      QString::fromUtf8(kXUrl))),
		this);
	links->setOpenExternalLinks(true);
	links->setTextInteractionFlags(Qt::TextBrowserInteraction);
	linksRow->addWidget(links);

	linksRow->addStretch();
	layout->addLayout(linksRow);

	layout->addWidget(makeSeparator(this));

	// Footer: utility action on the left, close on the right.
	auto *footer = new QHBoxLayout();
	footer->setSpacing(8);

	const QString obsInstallDir = obsInstallDirectory();
	auto *openObsFolder =
		new QPushButton(delaydeck::tr("About.OpenObsFolder"), this);
	openObsFolder->setEnabled(!obsInstallDir.isEmpty());
	openObsFolder->setToolTip(
		obsInstallDir.isEmpty()
			? delaydeck::tr("About.ObsFolderUnavailable")
			: obsInstallDir);
	connect(openObsFolder, &QPushButton::clicked, this, [obsInstallDir]() {
		if (!obsInstallDir.isEmpty()) {
			QDesktopServices::openUrl(
				QUrl::fromLocalFile(obsInstallDir));
		}
	});
	footer->addWidget(openObsFolder);

	footer->addStretch();

	auto *closeButton = new QPushButton(delaydeck::tr("About.Close"), this);
	closeButton->setDefault(true);
	connect(closeButton, &QPushButton::clicked, this, &QDialog::reject);
	footer->addWidget(closeButton);

	layout->addLayout(footer);

	relayout();
	startUpdateCheck();
}

void AboutDialog::startUpdateCheck()
{
	QPointer<AboutDialog> self(this);
	std::thread([self]() {
		const GithubReleaseCheckResult result = fetchLatestGithubRelease();
		QMetaObject::invokeMethod(
			qApp,
			[self, result]() {
				if (!self) {
					return;
				}
				self->applyReleaseCheckResult(result);
			},
			Qt::QueuedConnection);
	}).detach();
}

void AboutDialog::applyReleaseCheckResult(const GithubReleaseCheckResult &result)
{
	if (!result.success) {
		return;
	}

	if (isRemoteVersionNewer(result.tagName,
				 QString::fromUtf8(DELAYDECK_PLUGIN_VERSION))) {
		update_status_->setText(linkHtml(
			delaydeck::tr("About.UpdateAvailable").arg(result.tagName),
			result.htmlUrl));
		update_status_->setToolTip(delaydeck::tr("About.UpdateLink"));
	} else {
		update_status_->setText(
			QStringLiteral("<span style=\"color: gray;\">%1</span>")
				.arg(delaydeck::tr("About.UpToDate")
					     .toHtmlEscaped()));
		update_status_->setToolTip(QString());
	}

	update_status_->show();
	relayout();
}

void AboutDialog::relayout()
{
	adjustSize();
	setFixedSize(sizeHint());
}

} // namespace delaydeck
