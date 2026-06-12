#pragma once

#include "preflight/preflight-result.hpp"
#include "obs/slate-scene-controller.hpp"
#include "relay/relay-client.hpp"
#include "relay/relay-process.hpp"
#include "relay/relay-types.hpp"

#include <obs-frontend-api.h>

#include <obs.h>

#include <QCheckBox>
#include <QComboBox>
#include <QLabel>
#include <QPushButton>
#include <QSpinBox>
#include <QTimer>
#include <QToolButton>
#include <QWidget>

class DelayDeckDock final : public QWidget {
	Q_OBJECT

public:
	explicit DelayDeckDock(QWidget *parent = nullptr);
	~DelayDeckDock() override;

	void shutdown();
	void handleFrontendEvent(enum obs_frontend_event event);

	void loadSettings(obs_data_t *data);
	void saveSettings(obs_data_t *data) const;

	PreflightResult runPreflight();
	void setLastPreflightResult(const PreflightResult &result);
	const PreflightResult &lastPreflightResult() const
	{
		return last_preflight_result_;
	}

	void syncManagedRelayStartup();

private:
	static QString engineStatusText(RelayLinkState linkState,
					RelayProcessState processState);
	static QString operationLabel(const QString &operation);
	static QString relayStateLabel(const QString &state);
	static QString slateMessageLabel(const RelayStatus &status);
	static QString transitionText(const RelayStatus &status);
	static bool showsTransition(const RelayStatus &status);
	static bool delayToggleCheckedForStatus(const RelayStatus &status,
						bool startWithDelay);
	bool canEditDelayTarget() const;
	bool canOperateDelayToggle() const;
	bool slateScenesConfigured() const;
	bool hadActiveDelayBuffer() const;

	void applyHealth(const RelayHealth &health);
	void applyStatus(const RelayStatus &status);
	void applyLinkState(RelayLinkState state);
	void applyProcessState(RelayProcessState state);
	void applyControlFailed(const QString &code, const QString &message);
	void applyRequestFailed(const QString &operation, const QString &detail);
	void updateButtonStates();
	void updateProcessDisplay();
	void updateSummaryLabel(const RelayStatus *status = nullptr);
	void updateTransitionDisplay(const RelayStatus &status);
	void syncTransitionCountdown(const RelayStatus &status);
	void refreshTransitionLabel();
	void tickTransitionCountdown();
	void showError(const QString &text);
	void clearError();
	void startRelayClient();
	void updatePreflightDisplay(const PreflightResult &result);
	void refreshSceneSelectors();
	void updateSlateSceneState();
	void resetSummaryLabel();
	void onAdvancedToggled(bool visible);

	void syncDelayToggle(const RelayStatus &status);
	void maybeAutoEnableDelay();
	void onDelayToggleChanged(bool checked);
	void onDumpBufferClicked();
	void onRestartRelayClicked();
	void onEnableSlateSceneChanged();
	void onReturnSlateSceneChanged();
	void scheduleSettingsSave();
	void openSetupDialog();
	void openAboutDialog();
	void maybePromptSetup();
	void notifyObsStopDiscardedDelay();
	void applyDockSettings(int targetDelaySeconds, bool delayStream,
			       bool advancedVisible,
			       const QString &enableSlateScene,
			       const QString &returnSlateScene);

	RelayProcess *relay_process_;
	RelayClient *relay_client_;

	QLabel *summary_label_;
	QLabel *transition_label_;
	QLabel *error_label_;

	QWidget *advanced_panel_;
	QToolButton *advanced_toggle_button_;
	QSpinBox *target_delay_spin_;
	QComboBox *enable_slate_scene_combo_;
	QComboBox *return_slate_scene_combo_;
	QCheckBox *delay_toggle_;
	QPushButton *dump_buffer_button_;
	QPushButton *restart_relay_button_;
	QPushButton *setup_destination_button_;
	QToolButton *help_button_;

	SlateSceneController slate_scene_controller_;
	RelayLinkState link_state_ = RelayLinkState::Disconnected;
	RelayProcessState process_state_ = RelayProcessState::Idle;
	QString relay_state_;
	bool transition_pending_ = false;
	int active_delay_seconds_ = 0;
	qint64 last_status_updated_at_ms_ = -1;
	RelayStatus transition_status_;
	int transition_countdown_seconds_ = 0;
	QString transition_phase_key_;
	bool start_with_delay_ = false;
	bool auto_delay_triggered_ = false;
	bool loading_settings_ = false;
	bool setup_prompt_shown_ = false;
	QTimer settings_save_timer_;
	QTimer transition_countdown_timer_;
	PreflightResult last_preflight_result_{true, PreflightFailureCode::None, {}};
};
