#pragma once

#include <QDialog>

class QLineEdit;

namespace delaydeck {

class SetupDialog final : public QDialog {
	Q_OBJECT

public:
	explicit SetupDialog(QWidget *parent = nullptr);

	bool applySettings();

private:
	void prefillFromObs();

	QLineEdit *output_url_edit_;
	QLineEdit *stream_key_edit_;
};

} // namespace delaydeck
