#include "delaydeck-dock.hpp"

#include "locale/tr.hpp"
#include "relay/relay-client.hpp"

#include <QGridLayout>
#include <QHBoxLayout>
#include <QMessageBox>
#include <QVBoxLayout>

namespace {

QLabel *makeValueLabel(QWidget *parent)
{
	auto *label = new QLabel(tr("Value.Dash"), parent);
	label->setTextInteractionFlags(Qt::TextSelectableByMouse);
	return label;
}

} // namespace

QString DelayDeckDock::engineStatusText(RelayLinkState state)
{
	switch (state) {
	case RelayLinkState::Disconnected:
		return tr("Engine.Disconnected");
	case RelayLinkState::Unhealthy:
		return tr("Engine.Unhealthy");
	case RelayLinkState::AuthFailed:
		return tr("Engine.AuthFailed");
	case RelayLinkState::NoToken:
		return tr("Engine.NoToken");
	case RelayLinkState::Connected:
		return tr("Engine.Connected");
	}
	return tr("Engine.Disconnected");
}

QString DelayDeckDock::operationLabel(const QString &operation)
{
	if (operation == QStringLiteral("health")) {
		return tr("Operation.Health");
	}
	if (operation == QStringLiteral("status")) {
		return tr("Operation.Status");
	}
	return operation;
}

DelayDeckDock::DelayDeckDock(QWidget *parent) : QWidget(parent)
{
	relay_client_ = new RelayClient(this);

	auto *layout = new QVBoxLayout(this);

	engine_status_label_ = new QLabel(engineStatusText(RelayLinkState::Disconnected), this);
	engine_status_label_->setWordWrap(true);
	layout->addWidget(engine_status_label_);

	auto *status_grid = new QGridLayout();
	status_grid->setColumnStretch(1, 1);

	auto addRow = [&](int row, const QString &title, QLabel **value_out) {
		auto *title_label = new QLabel(title, this);
		*value_out = makeValueLabel(this);
		status_grid->addWidget(title_label, row, 0);
		status_grid->addWidget(*value_out, row, 1);
	};

	addRow(0, tr("Label.Health"), &health_label_);
	addRow(1, tr("Label.State"), &state_label_);
	addRow(2, tr("Label.TargetDelay"), &target_delay_label_);
	addRow(3, tr("Label.ActiveDelay"), &active_delay_label_);
	addRow(4, tr("Label.Buffer"), &buffer_label_);
	addRow(5, tr("Label.Input"), &input_label_);
	addRow(6, tr("Label.Output"), &output_label_);
	addRow(7, tr("Label.Slate"), &slate_label_);
	addRow(8, tr("Label.Countdown"), &countdown_label_);
	addRow(9, tr("Label.LastError"), &last_error_label_);

	layout->addLayout(status_grid);

	auto *delay_row = new QHBoxLayout();
	delay_row->addWidget(new QLabel(tr("Label.EnableDelaySeconds"), this));
	target_delay_spin_ = new QSpinBox(this);
	target_delay_spin_->setRange(1, 600);
	target_delay_spin_->setValue(30);
	delay_row->addWidget(target_delay_spin_);
	delay_row->addStretch();
	layout->addLayout(delay_row);

	auto *button_row = new QHBoxLayout();
	enable_delay_button_ = new QPushButton(tr("Button.EnableDelay"), this);
	return_live_button_ = new QPushButton(tr("Button.ReturnLive"), this);
	dump_buffer_button_ = new QPushButton(tr("Button.DumpBuffer"), this);

	button_row->addWidget(enable_delay_button_);
	button_row->addWidget(return_live_button_);
	button_row->addWidget(dump_buffer_button_);
	layout->addLayout(button_row);

	request_error_label_ = new QLabel(this);
	request_error_label_->setWordWrap(true);
	request_error_label_->setStyleSheet(QStringLiteral("color: #c0392b;"));
	request_error_label_->hide();
	layout->addWidget(request_error_label_);

	layout->addStretch();

	connect(relay_client_, &RelayClient::linkStateChanged, this,
		&DelayDeckDock::applyLinkState);
	connect(relay_client_, &RelayClient::healthUpdated, this,
		&DelayDeckDock::applyHealth);
	connect(relay_client_, &RelayClient::statusUpdated, this,
		&DelayDeckDock::applyStatus);
	connect(relay_client_, &RelayClient::controlFailed, this,
		&DelayDeckDock::applyControlFailed);
	connect(relay_client_, &RelayClient::requestFailed, this,
		&DelayDeckDock::applyRequestFailed);

	connect(enable_delay_button_, &QPushButton::clicked, this,
		&DelayDeckDock::onEnableDelayClicked);
	connect(return_live_button_, &QPushButton::clicked, this,
		&DelayDeckDock::onReturnLiveClicked);
	connect(dump_buffer_button_, &QPushButton::clicked, this,
		&DelayDeckDock::onDumpBufferClicked);

	updateButtonStates();
	relay_client_->start();
}

DelayDeckDock::~DelayDeckDock()
{
	relay_client_->stop();
}

void DelayDeckDock::applyHealth(const RelayHealth &health)
{
	const QString healthText =
		health.healthy ? tr("Value.Healthy") : tr("Value.Unhealthy");
	health_label_->setText(
		tr("Health.Format")
			.arg(healthText, health.mode, health.version,
			     health.uptimeSeconds));
}

void DelayDeckDock::applyStatus(const RelayStatus &status)
{
	relay_state_ = status.state;
	state_label_->setText(status.transitionPending
				      ? tr("Status.TransitionPending").arg(status.state)
				      : status.state);
	target_delay_label_->setText(
		tr("Value.Seconds").arg(status.targetDelaySeconds));
	active_delay_label_->setText(
		tr("Value.Seconds").arg(status.activeDelaySeconds));
	buffer_label_->setText(
		tr("Value.Percent").arg(status.bufferUsagePercent, 0, 'f', 1));
	input_label_->setText(status.inputState);
	output_label_->setText(status.outputState);

	if (status.slateMessage.isEmpty()) {
		slate_label_->setText(tr("Value.None"));
	} else {
		slate_label_->setText(status.slateMessage);
	}
	if (status.countdownSeconds > 0) {
		countdown_label_->setText(
			tr("Value.Seconds").arg(status.countdownSeconds));
	} else {
		countdown_label_->setText(tr("Value.Dash"));
	}

	if (status.lastError.isEmpty()) {
		last_error_label_->setText(tr("Value.None"));
	} else {
		last_error_label_->setText(status.lastError);
	}

	request_error_label_->hide();
	updateButtonStates();
}

void DelayDeckDock::applyLinkState(RelayLinkState state)
{
	link_state_ = state;
	engine_status_label_->setText(engineStatusText(state));

	if (state != RelayLinkState::Connected &&
	    state != RelayLinkState::Unhealthy) {
		health_label_->setText(tr("Value.Dash"));
	}

	if (state != RelayLinkState::Connected) {
		state_label_->setText(tr("Value.Dash"));
		target_delay_label_->setText(tr("Value.Dash"));
		active_delay_label_->setText(tr("Value.Dash"));
		buffer_label_->setText(tr("Value.Dash"));
		input_label_->setText(tr("Value.Dash"));
		output_label_->setText(tr("Value.Dash"));
		slate_label_->setText(tr("Value.Dash"));
		countdown_label_->setText(tr("Value.Dash"));
		last_error_label_->setText(tr("Value.Dash"));
		relay_state_.clear();
	}

	updateButtonStates();
}

void DelayDeckDock::applyControlFailed(const QString &code,
				       const QString &message)
{
	QString detail = message;
	if (code == QStringLiteral("relay_unavailable")) {
		detail = engineStatusText(link_state_);
	}

	request_error_label_->setText(
		tr("Error.ControlFailed").arg(code, detail));
	request_error_label_->show();
}

void DelayDeckDock::applyRequestFailed(const QString &operation,
				       const QString &detail)
{
	if (link_state_ == RelayLinkState::Connected &&
	    operation == QStringLiteral("health")) {
		return;
	}

	request_error_label_->setText(
		tr("Error.RequestFailed").arg(operationLabel(operation), detail));
	request_error_label_->show();
}

void DelayDeckDock::updateButtonStates()
{
	const bool connected = link_state_ == RelayLinkState::Connected;
	enable_delay_button_->setEnabled(connected);
	return_live_button_->setEnabled(connected);
	dump_buffer_button_->setEnabled(connected);
	target_delay_spin_->setEnabled(connected);
}

void DelayDeckDock::onEnableDelayClicked()
{
	relay_client_->enableDelay(target_delay_spin_->value());
}

void DelayDeckDock::onReturnLiveClicked()
{
	relay_client_->returnLive();
}

void DelayDeckDock::onDumpBufferClicked()
{
	const auto answer = QMessageBox::question(
		this, tr("DumpBuffer.Title"), tr("DumpBuffer.Message"),
		QMessageBox::Yes | QMessageBox::No, QMessageBox::No);
	if (answer != QMessageBox::Yes) {
		return;
	}
	relay_client_->dumpBuffer();
}
