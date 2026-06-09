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
	static QString relayStateLabel(const QString &state);
	static QString slateMessageLabel(const RelayStatus &status);
	static QString transitionText(const RelayStatus &status);
	static bool showsTransition(const RelayStatus &status);
	bool canEditDelayTarget() const;

	void applyHealth(const RelayHealth &health);
	void applyStatus(const RelayStatus &status);
	void applyLinkState(RelayLinkState state);
	void applyProcessState(RelayProcessState state);
	void applyControlFailed(const QString &code, const QString &message);
	void applyRequestFailed(const QString &operation, const QString &detail);
	void updateButtonStates();
	void updateProcessDisplay();
	void updateSummaryLabel();
	void updateTransitionDisplay(const RelayStatus &status);
	void showError(const QString &text);
	void clearError();
	void startRelayClient();
	void updatePreflightDisplay(const PreflightResult &result);
	void refreshSceneSelectors();
	void resetSummaryLabel();
	void onAdvancedToggled(bool visible);

	void onEnableDelayClicked();
	void onReturnLiveClicked();
	void onDumpBufferClicked();
	void onRestartRelayClicked();
	void onEnableSlateSceneChanged();
	void onReturnSlateSceneChanged();

	RelayProcess *relay_process_;
	RelayClient *relay_client_;

	QLabel *summary_label_;
	QLabel *transition_label_;
	QLabel *error_label_;

	QWidget *advanced_panel_;
	QPushButton *advanced_toggle_button_;
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
	bool transition_pending_ = false;
	int active_delay_seconds_ = 0;
	PreflightResult last_preflight_result_{true, PreflightFailureCode::None, {}};
};
