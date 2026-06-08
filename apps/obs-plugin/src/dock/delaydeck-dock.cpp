#include "delaydeck-dock.hpp"

#include <QVBoxLayout>

DelayDeckDock::DelayDeckDock(QWidget *parent) : QWidget(parent)
{
	auto *layout = new QVBoxLayout(this);

	engine_status_label_ = new QLabel(QStringLiteral("Engine: disconnected"), this);
	layout->addWidget(engine_status_label_);

	setLayout(layout);
}
