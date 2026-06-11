#pragma once

#include <QDialog>

namespace delaydeck {

class AboutDialog final : public QDialog {
	Q_OBJECT

public:
	explicit AboutDialog(QWidget *parent = nullptr);
};

} // namespace delaydeck
