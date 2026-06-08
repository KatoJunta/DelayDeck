#pragma once

#include <QLabel>
#include <QWidget>

class DelayDeckDock final : public QWidget {
	Q_OBJECT

public:
	explicit DelayDeckDock(QWidget *parent = nullptr);

private:
	QLabel *engine_status_label_;
};
