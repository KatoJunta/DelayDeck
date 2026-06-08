#pragma once

#include "relay/relay-client.hpp"
#include "relay/relay-process.hpp"
#include "relay/relay-types.hpp"

#include <QLabel>
#include <QPushButton>
#include <QSpinBox>
#include <QWidget>

class DelayDeckDock final : public QWidget {
	Q_OBJECT

public:
	explicit DelayDeckDock(QWidget *parent = nullptr);
	~DelayDeckDock() override;

	void shutdown();

private:
	static QString engineStatusText(RelayLinkState linkState,
					RelayProcessState processState);
	static QString operationLabel(const QString &operation);

	void applyHealth(const RelayHealth &health);
	void applyStatus(const RelayStatus &status);
	void applyLinkState(RelayLinkState state);
	void applyProcessState(RelayProcessState state);
	void applyControlFailed(const QString &code, const QString &message);
	void applyRequestFailed(const QString &operation, const QString &detail);
	void updateButtonStates();
	void updateProcessDisplay();
	void startRelayClient();

	void onEnableDelayClicked();
	void onReturnLiveClicked();
	void onDumpBufferClicked();
	void onRestartRelayClicked();

	RelayProcess *relay_process_;
	RelayClient *relay_client_;

	QLabel *engine_status_label_;
	QLabel *process_id_label_;
	QLabel *health_label_;
	QLabel *state_label_;
	QLabel *target_delay_label_;
	QLabel *active_delay_label_;
	QLabel *buffer_label_;
	QLabel *input_label_;
	QLabel *output_label_;
	QLabel *slate_label_;
	QLabel *countdown_label_;
	QLabel *last_error_label_;
	QLabel *request_error_label_;

	QSpinBox *target_delay_spin_;
	QPushButton *enable_delay_button_;
	QPushButton *return_live_button_;
	QPushButton *dump_buffer_button_;
	QPushButton *restart_relay_button_;

	RelayLinkState link_state_ = RelayLinkState::Disconnected;
	RelayProcessState process_state_ = RelayProcessState::Idle;
	QString relay_state_;
};
