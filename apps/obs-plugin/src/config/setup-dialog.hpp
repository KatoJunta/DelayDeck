#pragma once

#include <QDialog>

class QComboBox;
class QLineEdit;

namespace delaydeck {

class SetupDialog final : public QDialog {
	Q_OBJECT

public:
	explicit SetupDialog(QWidget *parent = nullptr);

	bool applySettings();

private:
	void populatePlatformCombo();
	void syncPlatformSelectionFromUrl();
	void onPlatformChanged(int index);
	void prefillFromObs();

	QComboBox *platform_combo_;
	QLineEdit *output_url_edit_;
	QLineEdit *stream_key_edit_;
};

} // namespace delaydeck
