#include "config/setup-dialog.hpp"

#include "config/relay-platform-presets.hpp"
#include "config/relay-settings.hpp"
#include "config/relay-destination-url.hpp"
#include "locale/tr.hpp"
#include "obs/streaming-setup.hpp"

#include <obs-frontend-api.h>

#include <QComboBox>
#include <QDialogButtonBox>
#include <QFormLayout>
#include <QLabel>
#include <QLineEdit>
#include <QMessageBox>
#include <QVBoxLayout>

namespace delaydeck {

SetupDialog::SetupDialog(QWidget *parent) : QDialog(parent)
{
	setWindowTitle(delaydeck::tr("Setup.Title"));
	setModal(true);

	auto *layout = new QVBoxLayout(this);

	auto *intro = new QLabel(delaydeck::tr("Setup.Intro"), this);
	intro->setWordWrap(true);
	layout->addWidget(intro);

	auto *form = new QFormLayout();
	platform_combo_ = new QComboBox(this);
	output_url_edit_ = new QLineEdit(this);
	output_url_edit_->setPlaceholderText(
		QStringLiteral("rtmp://a.rtmp.youtube.com/live2"));
	stream_key_edit_ = new QLineEdit(this);
	stream_key_edit_->setEchoMode(QLineEdit::Password);
	form->addRow(delaydeck::tr("Setup.Platform"), platform_combo_);
	form->addRow(delaydeck::tr("Setup.OutputUrl"), output_url_edit_);
	form->addRow(delaydeck::tr("Setup.StreamKey"), stream_key_edit_);
	layout->addLayout(form);

	populatePlatformCombo();
	connect(platform_combo_, qOverload<int>(&QComboBox::currentIndexChanged), this,
		&SetupDialog::onPlatformChanged);

	auto *obsNote = new QLabel(delaydeck::tr("Setup.ObsNote"), this);
	obsNote->setWordWrap(true);
	layout->addWidget(obsNote);

	auto *buttons =
		new QDialogButtonBox(QDialogButtonBox::Save | QDialogButtonBox::Cancel,
				     this);
	connect(buttons, &QDialogButtonBox::accepted, this, [this]() {
		if (applySettings()) {
			accept();
		}
	});
	connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
	layout->addWidget(buttons);

	prefillFromObs();

	const RelayDestination saved = RelaySettings::instance().destination();
	if (!saved.outputUrl.isEmpty()) {
		output_url_edit_->setText(saved.outputUrl);
	}
	if (!saved.streamKey.isEmpty()) {
		stream_key_edit_->setText(saved.streamKey);
	}

	syncPlatformSelectionFromUrl();
}

void SetupDialog::populatePlatformCombo()
{
	platform_combo_->clear();

	const QVector<PlatformPreset> presets = platformPresets();
	for (const PlatformPreset &preset : presets) {
		platform_combo_->addItem(delaydeck::tr(preset.localeKey),
					   preset.outputUrl);
	}
}

void SetupDialog::syncPlatformSelectionFromUrl()
{
	const int index = platformPresetIndexForUrl(output_url_edit_->text());
	platform_combo_->blockSignals(true);
	platform_combo_->setCurrentIndex(index);
	platform_combo_->blockSignals(false);
}

void SetupDialog::onPlatformChanged(int index)
{
	if (index < 0) {
		return;
	}

	const QString presetUrl = platform_combo_->itemData(index).toString();
	if (presetUrl.isEmpty()) {
		return;
	}

	output_url_edit_->setText(presetUrl);
}

void SetupDialog::prefillFromObs()
{
	const ObsStreamingSnapshot snapshot = readCurrentStreaming();
	if (!snapshot.canPrefillDestination()) {
		return;
	}

	if (output_url_edit_->text().trimmed().isEmpty()) {
		output_url_edit_->setText(snapshot.serverUrl);
	}
	if (stream_key_edit_->text().isEmpty()) {
		stream_key_edit_->setText(snapshot.streamKey);
	}
}

bool SetupDialog::applySettings()
{
	if (obs_frontend_streaming_active()) {
		QMessageBox::warning(this, delaydeck::tr("Setup.Title"),
				     delaydeck::tr("Setup.Error.StreamingActive"));
		return false;
	}

	const QString outputUrl = output_url_edit_->text().trimmed();
	const QString streamKey = stream_key_edit_->text().trimmed();

	if (outputUrl.isEmpty() || streamKey.isEmpty()) {
		QMessageBox::warning(this, delaydeck::tr("Setup.Title"),
				     delaydeck::tr("Setup.Error.MissingFields"));
		return false;
	}

	if (!isValidRelayOutputUrl(outputUrl)) {
		QMessageBox::warning(this, delaydeck::tr("Setup.Title"),
				     delaydeck::tr("Setup.Error.InvalidUrl"));
		return false;
	}

	const QString confirmMessage =
		delaydeck::tr("Setup.ConfirmMessage")
			.arg(outputUrl, QString::fromUtf8(kLocalRelayServerUrl));
	const auto answer = QMessageBox::question(
		this, delaydeck::tr("Setup.ConfirmTitle"), confirmMessage,
		QMessageBox::Yes | QMessageBox::No, QMessageBox::No);
	if (answer != QMessageBox::Yes) {
		return false;
	}

	if (!RelaySettings::instance().saveDestination(outputUrl, streamKey)) {
		QMessageBox::critical(this, delaydeck::tr("Setup.Title"),
				      delaydeck::tr("Setup.Error.SaveFailed"));
		return false;
	}

	if (!applyLocalRelayIngest()) {
		QMessageBox::critical(this, delaydeck::tr("Setup.Title"),
				      delaydeck::tr("Setup.Error.ObsApplyFailed"));
		return false;
	}

	return true;
}

} // namespace delaydeck
