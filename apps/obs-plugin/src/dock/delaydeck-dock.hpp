#pragma once

#include "preflight/preflight-result.hpp"
#include "obs/slate-scene-controller.hpp"
#include "relay/relay-client.hpp"
#include "relay/relay-process.hpp"
#include "relay/relay-types.hpp"

#include <obs-frontend-api.h>

#include <QComboBox>
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
	void handleFrontendEvent(enum obs_frontend_event event);

	PreflightResult runPreflight();
	void setLastPreflightResult(const PreflightResult &result);
	const PreflightResult &lastPreflightResult() const
	{
		return last_preflight_result_;
	}

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
	void updatePreflightDisplay(const PreflightResult &result);
	void refreshSceneSelectors();

	void onEnableDelayClicked();
	void onReturnLiveClicked();
	void onDumpBufferClicked();
	void onRestartRelayClicked();
	void onEnableSlateSceneChanged();
	void onReturnSlateSceneChanged();

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
	QLabel *preflight_label_;

	QSpinBox *target_delay_spin_;
	QComboBox *enable_slate_scene_combo_;
	QComboBox *return_slate_scene_combo_;
	QPushButton *enable_delay_button_;
	QPushButton *return_live_button_;
	QPushButton *dump_buffer_button_;
	QPushButton *restart_relay_button_;

	SlateSceneController slate_scene_controller_;
	RelayLinkState link_state_ = RelayLinkState::Disconnected;
	RelayProcessState process_state_ = RelayProcessState::Idle;
	QString relay_state_;
	PreflightResult last_preflight_result_;
};
