#include "dock/about-dialog.hpp"

#include "locale/tr.hpp"
#include "version.hpp"

#include <QDialogButtonBox>
#include <QFont>
#include <QLabel>
#include <QVBoxLayout>

namespace delaydeck {

namespace {

constexpr const char *kGithubUrl = "https://github.com/KatoJunta/DelayDeck";
constexpr const char *kXUrl = "https://x.com/KatoJunta";

QLabel *makeLinkLabel(const QString &text, const char *url, QWidget *parent)
{
	auto *label = new QLabel(
		QStringLiteral("<a href=\"%1\">%2</a>")
			.arg(QString::fromUtf8(url), text.toHtmlEscaped()),
		parent);
	label->setOpenExternalLinks(true);
	label->setTextInteractionFlags(Qt::TextBrowserInteraction);
	return label;
}

} // namespace

AboutDialog::AboutDialog(QWidget *parent) : QDialog(parent)
{
	setWindowTitle(delaydeck::tr("About.Title"));
	setModal(true);

	auto *layout = new QVBoxLayout(this);
	layout->setSpacing(10);

	auto *title = new QLabel(QStringLiteral("DelayDeck"), this);
	QFont titleFont = title->font();
	titleFont.setPointSizeF(titleFont.pointSizeF() * 1.4);
	titleFont.setBold(true);
	title->setFont(titleFont);
	layout->addWidget(title);

	auto *version = new QLabel(
		delaydeck::tr("About.Version")
			.arg(QString::fromUtf8(DELAYDECK_PLUGIN_VERSION)),
		this);
	version->setTextInteractionFlags(Qt::TextSelectableByMouse);
	layout->addWidget(version);

	auto *description = new QLabel(delaydeck::tr("About.Description"), this);
	description->setWordWrap(true);
	layout->addWidget(description);

	layout->addSpacing(4);
	layout->addWidget(
		makeLinkLabel(delaydeck::tr("About.GithubLink"), kGithubUrl, this));
	layout->addWidget(makeLinkLabel(delaydeck::tr("About.XLink"), kXUrl, this));
	layout->addSpacing(4);

	auto *buttons = new QDialogButtonBox(QDialogButtonBox::Close, this);
	connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
	layout->addWidget(buttons);

	setFixedSize(sizeHint());
}

} // namespace delaydeck
