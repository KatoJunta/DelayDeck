#include "delaydeck-dock.hpp"

#include "config/dock-settings.hpp"
#include "locale/tr.hpp"
#include "preflight/preflight-checker.hpp"
#include "preflight/preflight-dialog.hpp"
#include "preflight/streaming-guard.hpp"
#include "relay/relay-client.hpp"

#include <obs-frontend-api.h>
#include <obs.hpp>

#include <QGridLayout>
#include <QHash>
#include <QHBoxLayout>
#include <QMessageBox>
#include <QSignalBlocker>
#include <QTimer>
#include <QVBoxLayout>

namespace {

void populateSceneCombo(QComboBox *combo, const QStringList &sceneNames)
{
	const QString previous = combo->currentData().toString();
	const QSignalBlocker blocker(combo);

	combo->clear();
	combo->addItem(delaydeck::tr("SlateScene.None"), QString());
	for (const QString &name : sceneNames) {
		combo->addItem(name, name);
	}

	const int index = combo->findData(previous);
	combo->setCurrentIndex(index >= 0 ? index : 0);
}

enum class SlateDisplayKind {
	None,
	EnableDelay,
	Draining,
	ReturningLive,
};

bool slateMessageStartsWith(const QString &message, const char *prefix)
{
	return message.startsWith(QString::fromUtf8(prefix));
}

SlateDisplayKind slateDisplayKind(const RelayStatus &status)
{
	if (status.state == QStringLiteral("BUFFERING_TO_DELAY")) {
		return SlateDisplayKind::EnableDelay;
	}
	if (status.state == QStringLiteral("RETURNING_TO_REALTIME")) {
		return SlateDisplayKind::Draining;
	}
	if (status.state == QStringLiteral("DUMPING")) {
		return SlateDisplayKind::ReturningLive;
	}

	if (status.state == QStringLiteral("SAFE_HOLD")) {
		if (slateMessageStartsWith(status.slateMessage,
					   "Getting ready to delay the stream") ||
		    slateMessageStartsWith(status.slateMessage, "Delay activating")) {
			return SlateDisplayKind::EnableDelay;
		}
		if (slateMessageStartsWith(status.slateMessage,
					   "Finishing delayed content") ||
		    slateMessageStartsWith(status.slateMessage,
					   "Playing buffered content")) {
			return SlateDisplayKind::Draining;
		}
		if (slateMessageStartsWith(status.slateMessage, "Switching back to live") ||
		    slateMessageStartsWith(status.slateMessage, "Returning to realtime")) {
			return SlateDisplayKind::ReturningLive;
		}
	}

	if (status.slateMessage.isEmpty() || status.countdownSeconds <= 0) {
		return SlateDisplayKind::None;
	}

	if (slateMessageStartsWith(status.slateMessage,
				   "Getting ready to delay the stream") ||
	    slateMessageStartsWith(status.slateMessage, "Delay activating")) {
		return SlateDisplayKind::EnableDelay;
	}
	if (slateMessageStartsWith(status.slateMessage, "Finishing delayed content") ||
	    slateMessageStartsWith(status.slateMessage, "Playing buffered content")) {
		return SlateDisplayKind::Draining;
	}
	if (slateMessageStartsWith(status.slateMessage, "Switching back to live") ||
	    slateMessageStartsWith(status.slateMessage, "Returning to realtime")) {
		return SlateDisplayKind::ReturningLive;
	}

	return SlateDisplayKind::None;
}

bool isTransitionState(const QString &state)
{
	return state == QStringLiteral("BUFFERING_TO_DELAY") ||
	       state == QStringLiteral("SAFE_HOLD") ||
	       state == QStringLiteral("RETURNING_TO_REALTIME") ||
	       state == QStringLiteral("DUMPING");
}

void selectComboItem(QComboBox *combo, const QString &value)
{
	const int index = combo->findData(value);
	combo->setCurrentIndex(index >= 0 ? index : 0);
}

} // namespace

QString DelayDeckDock::engineStatusText(RelayLinkState linkState,
					RelayProcessState processState)
{
	if (processState != RelayProcessState::Unmanaged) {
		switch (processState) {
		case RelayProcessState::Starting:
			return delaydeck::tr("Engine.Starting");
		case RelayProcessState::Stopping:
			return delaydeck::tr("Engine.Stopping");
		case RelayProcessState::FailedToStart:
			return delaydeck::tr("Engine.ProcessFailed");
		case RelayProcessState::Crashed:
			return delaydeck::tr("Engine.ProcessCrashed");
		case RelayProcessState::Running:
			if (linkState == RelayLinkState::Connected) {
				return delaydeck::tr("Engine.Connected");
			}
			if (linkState == RelayLinkState::Unhealthy) {
				return delaydeck::tr("Engine.Unhealthy");
			}
			if (linkState == RelayLinkState::AuthFailed) {
				return delaydeck::tr("Engine.AuthFailed");
			}
			return delaydeck::tr("Engine.Disconnected");
		default:
			break;
		}
	}

	switch (linkState) {
	case RelayLinkState::Disconnected:
		return delaydeck::tr("Engine.Disconnected");
	case RelayLinkState::Unhealthy:
		return delaydeck::tr("Engine.Unhealthy");
	case RelayLinkState::AuthFailed:
		return delaydeck::tr("Engine.AuthFailed");
	case RelayLinkState::NoToken:
		return delaydeck::tr("Engine.NoToken");
	case RelayLinkState::Connected:
		return delaydeck::tr("Engine.Connected");
	}
	return delaydeck::tr("Engine.Disconnected");
}

QString DelayDeckDock::operationLabel(const QString &operation)
{
	if (operation == QStringLiteral("health")) {
		return delaydeck::tr("Operation.Health");
	}
	if (operation == QStringLiteral("status")) {
		return delaydeck::tr("Operation.Status");
	}
	return operation;
}

QString DelayDeckDock::relayStateLabel(const QString &state)
{
	static const QHash<QString, const char *> labels = {
		{QStringLiteral("REALTIME"), "State.Realtime"},
		{QStringLiteral("BUFFERING_TO_DELAY"), "State.BufferingToDelay"},
		{QStringLiteral("SAFE_HOLD"), "State.SafeHold"},
		{QStringLiteral("DELAYED"), "State.Delayed"},
		{QStringLiteral("RETURNING_TO_REALTIME"), "State.ReturningToRealtime"},
		{QStringLiteral("DUMPING"), "State.Dumping"},
		{QStringLiteral("INGESTING"), "State.Ingesting"},
		{QStringLiteral("READY"), "State.Ready"},
		{QStringLiteral("STARTING"), "State.Starting"},
		{QStringLiteral("STOPPED"), "State.Stopped"},
		{QStringLiteral("ERROR"), "State.Error"},
	};

	const auto it = labels.find(state);
	if (it != labels.end()) {
		return delaydeck::tr(it.value());
	}
	return state;
}

QString DelayDeckDock::slateMessageLabel(const RelayStatus &status)
{
	switch (slateDisplayKind(status)) {
	case SlateDisplayKind::EnableDelay:
		return delaydeck::tr("Slate.EnableDelay");
	case SlateDisplayKind::Draining:
		return delaydeck::tr("Slate.Draining");
	case SlateDisplayKind::ReturningLive:
		return delaydeck::tr("Slate.ReturningLive");
	case SlateDisplayKind::None:
		break;
	}

	if (!status.slateMessage.isEmpty()) {
		return status.slateMessage;
	}
	return {};
}

bool showsDelayValueForState(const QString &state, int activeDelaySeconds)
{
	if (activeDelaySeconds > 0) {
		return true;
	}

	return state == QStringLiteral("DELAYED") ||
	       state == QStringLiteral("BUFFERING_TO_DELAY") ||
	       state == QStringLiteral("RETURNING_TO_REALTIME");
}

bool DelayDeckDock::showsTransition(const RelayStatus &status)
{
	if (slateDisplayKind(status) != SlateDisplayKind::None) {
		return true;
	}

	return isTransitionState(status.state) && status.countdownSeconds > 0;
}

bool DelayDeckDock::delayToggleCheckedForStatus(const RelayStatus &status,
						bool startWithDelay)
{
	if (status.state == QStringLiteral("DELAYED") ||
	    status.state == QStringLiteral("BUFFERING_TO_DELAY")) {
		return true;
	}
	if (status.state == QStringLiteral("RETURNING_TO_REALTIME") ||
	    status.state == QStringLiteral("DUMPING")) {
		return false;
	}
	if (status.state == QStringLiteral("SAFE_HOLD")) {
		switch (slateDisplayKind(status)) {
		case SlateDisplayKind::EnableDelay:
			return true;
		case SlateDisplayKind::ReturningLive:
		case SlateDisplayKind::Draining:
			return false;
		case SlateDisplayKind::None:
			break;
		}
	}
	if (status.state == QStringLiteral("REALTIME")) {
		return startWithDelay;
	}
	return startWithDelay;
}

QString DelayDeckDock::transitionText(const RelayStatus &status)
{
	const QString message = slateMessageLabel(status);
	if (!message.isEmpty() && status.countdownSeconds > 0) {
		return delaydeck::tr("Transition.WithCountdown")
			.arg(message)
			.arg(status.countdownSeconds);
	}
	if (!message.isEmpty()) {
		return message;
	}
	if (status.countdownSeconds > 0) {
		return delaydeck::tr("Transition.CountdownOnly")
			.arg(status.countdownSeconds);
	}
	return {};
}

DelayDeckDock::DelayDeckDock(QWidget *parent) : QWidget(parent)
{
	relay_process_ = new RelayProcess(this);
	relay_client_ = new RelayClient(this);

	auto *layout = new QVBoxLayout(this);
	layout->setSpacing(8);

	summary_label_ = new QLabel(this);
	summary_label_->setWordWrap(true);
	summary_label_->setTextInteractionFlags(Qt::TextSelectableByMouse);
	layout->addWidget(summary_label_);

	transition_label_ = new QLabel(this);
	transition_label_->setWordWrap(true);
	transition_label_->hide();
	layout->addWidget(transition_label_);

	auto *control_row = new QHBoxLayout();
	target_delay_spin_ = new QSpinBox(this);
	target_delay_spin_->setRange(1, 600);
	target_delay_spin_->setValue(30);
	target_delay_spin_->setSuffix(delaydeck::tr("Value.DelaySuffix"));
	control_row->addWidget(target_delay_spin_);
	delay_toggle_ = new QCheckBox(delaydeck::tr("Toggle.DelayStream"), this);
	control_row->addWidget(delay_toggle_);
	layout->addLayout(control_row);

	dump_buffer_button_ = new QPushButton(delaydeck::tr("Button.DumpBuffer"), this);
	layout->addWidget(dump_buffer_button_);

	restart_relay_button_ = new QPushButton(delaydeck::tr("Button.RestartRelay"), this);
	layout->addWidget(restart_relay_button_);

	advanced_toggle_button_ = new QPushButton(delaydeck::tr("Section.ShowAdvanced"), this);
	advanced_toggle_button_->setCheckable(true);
	layout->addWidget(advanced_toggle_button_);

	advanced_panel_ = new QWidget(this);
	auto *advanced_layout = new QVBoxLayout(advanced_panel_);
	advanced_layout->setContentsMargins(0, 0, 0, 0);

	advanced_layout->addWidget(new QLabel(delaydeck::tr("Section.Destination"), this));

	auto *scene_grid = new QGridLayout();
	scene_grid->setColumnStretch(1, 1);
	enable_slate_scene_combo_ = new QComboBox(advanced_panel_);
	return_slate_scene_combo_ = new QComboBox(advanced_panel_);
	scene_grid->addWidget(new QLabel(delaydeck::tr("Label.EnableSlateScene"), advanced_panel_),
			      0, 0);
	scene_grid->addWidget(enable_slate_scene_combo_, 0, 1);
	scene_grid->addWidget(new QLabel(delaydeck::tr("Label.ReturnSlateScene"), advanced_panel_),
			      1, 0);
	scene_grid->addWidget(return_slate_scene_combo_, 1, 1);
	advanced_layout->addLayout(scene_grid);

	advanced_panel_->setVisible(false);
	layout->addWidget(advanced_panel_);

	error_label_ = new QLabel(this);
	error_label_->setWordWrap(true);
	error_label_->setStyleSheet(QStringLiteral("color: #c0392b;"));
	error_label_->hide();
	layout->addWidget(error_label_);

	layout->addStretch();

	connect(relay_process_, &RelayProcess::stateChanged, this,
		&DelayDeckDock::applyProcessState);
	connect(relay_process_, &RelayProcess::processDied, this, [this]() {
		relay_client_->suspendTraffic();
		updateProcessDisplay();
	});
	connect(relay_process_, &RelayProcess::credentialsReady, this,
		[this](const QString &apiBaseUrl, const QString &sessionToken) {
			relay_client_->setCredentials(apiBaseUrl, sessionToken);
		});

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

	connect(delay_toggle_, &QCheckBox::toggled, this,
		&DelayDeckDock::onDelayToggleChanged);
	connect(dump_buffer_button_, &QPushButton::clicked, this,
		&DelayDeckDock::onDumpBufferClicked);
	connect(restart_relay_button_, &QPushButton::clicked, this,
		&DelayDeckDock::onRestartRelayClicked);
	connect(advanced_toggle_button_, &QPushButton::toggled, this,
		&DelayDeckDock::onAdvancedToggled);
	connect(target_delay_spin_, qOverload<int>(&QSpinBox::valueChanged), this,
		[this](int) { scheduleSettingsSave(); });
	connect(enable_slate_scene_combo_, qOverload<int>(&QComboBox::currentIndexChanged),
		this, [this](int) { onEnableSlateSceneChanged(); });
	connect(return_slate_scene_combo_, qOverload<int>(&QComboBox::currentIndexChanged),
		this, [this](int) { onReturnSlateSceneChanged(); });

	settings_save_timer_.setSingleShot(true);
	settings_save_timer_.setInterval(400);
	connect(&settings_save_timer_, &QTimer::timeout, this, []() {
		obs_frontend_save();
	});

	applyDockSettings(30, false, true, {}, {});
	resetSummaryLabel();
	refreshSceneSelectors();
	updateButtonStates();

	if (relay_process_->isManaged()) {
		process_state_ = RelayProcessState::Idle;
		relay_process_->startRelay();
	} else {
		process_state_ = RelayProcessState::Unmanaged;
		startRelayClient();
	}

	StreamingGuard::install(this);
}

void DelayDeckDock::handleFrontendEvent(enum obs_frontend_event event)
{
	if (event == OBS_FRONTEND_EVENT_STREAMING_STARTED) {
		QTimer::singleShot(0, this, [this]() { maybeAutoEnableDelay(); });
		return;
	}

	if (event == OBS_FRONTEND_EVENT_STREAMING_STOPPED) {
		auto_delay_triggered_ = false;
		return;
	}

	if (event != OBS_FRONTEND_EVENT_FINISHED_LOADING &&
	    event != OBS_FRONTEND_EVENT_SCENE_LIST_CHANGED &&
	    event != OBS_FRONTEND_EVENT_SCENE_COLLECTION_CHANGED) {
		return;
	}

	QTimer::singleShot(0, this, [this]() { refreshSceneSelectors(); });
}

void DelayDeckDock::shutdown()
{
	settings_save_timer_.stop();
	obs_frontend_save();
	StreamingGuard::uninstall();
	slate_scene_controller_.clear();
	relay_client_->stop();
	relay_process_->shutdown();
}

PreflightResult DelayDeckDock::runPreflight()
{
	return PreflightChecker::run(process_state_, relay_process_->isManaged(),
				     relay_client_->apiBaseUrl(),
				     relay_process_->sessionToken());
}

void DelayDeckDock::setLastPreflightResult(const PreflightResult &result)
{
	last_preflight_result_ = result;
	updatePreflightDisplay(result);
}

void DelayDeckDock::updatePreflightDisplay(const PreflightResult &result)
{
	if (result.ok || result.code == PreflightFailureCode::None) {
		clearError();
		return;
	}

	showError(delaydeck::tr("Preflight.Failed")
			  .arg(PreflightDialog::messageFor(result)));
}

DelayDeckDock::~DelayDeckDock()
{
	shutdown();
}

void DelayDeckDock::applyHealth(const RelayHealth &health)
{
	Q_UNUSED(health);
}

void DelayDeckDock::applyStatus(const RelayStatus &status)
{
	if (status.updatedAtMs >= 0 && last_status_updated_at_ms_ >= 0 &&
	    status.updatedAtMs < last_status_updated_at_ms_) {
		return;
	}
	if (status.updatedAtMs >= 0) {
		last_status_updated_at_ms_ = status.updatedAtMs;
	}

	relay_state_ = status.state;
	transition_pending_ = status.transitionPending;
	active_delay_seconds_ = status.activeDelaySeconds;
	updateSummaryLabel();
	updateTransitionDisplay(status);

	if (!status.lastError.isEmpty()) {
		showError(status.lastError);
	} else if (!last_preflight_result_.ok &&
		   last_preflight_result_.code != PreflightFailureCode::None) {
		updatePreflightDisplay(last_preflight_result_);
	} else {
		clearError();
	}

	slate_scene_controller_.applyStatus(status);
	syncDelayToggle(status);
	updateButtonStates();
	maybeAutoEnableDelay();
}

void DelayDeckDock::applyProcessState(RelayProcessState state)
{
	process_state_ = state;
	updateProcessDisplay();
	updateButtonStates();

	if (state == RelayProcessState::Running) {
		startRelayClient();
		return;
	}

	if (state == RelayProcessState::Starting) {
		relay_client_->suspendTraffic();
		return;
	}

	if (state == RelayProcessState::Crashed ||
	    state == RelayProcessState::FailedToStart) {
		relay_client_->suspendTraffic();
		if (!relay_process_->lastError().isEmpty()) {
			showError(delaydeck::tr("Error.RelayProcess").arg(
				relay_process_->lastError()));
		}
	}

	if (state == RelayProcessState::Stopping || state == RelayProcessState::Idle) {
		relay_client_->suspendTraffic();
	}
}

void DelayDeckDock::applyLinkState(RelayLinkState state)
{
	link_state_ = state;

	if (state != RelayLinkState::Connected) {
		relay_state_.clear();
		transition_pending_ = false;
		active_delay_seconds_ = 0;
		last_status_updated_at_ms_ = -1;
		transition_label_->hide();
		slate_scene_controller_.clear();
	}

	updateSummaryLabel();
	{
		const QSignalBlocker blocker(delay_toggle_);
		delay_toggle_->setChecked(start_with_delay_);
	}
	updateButtonStates();
}

void DelayDeckDock::applyControlFailed(const QString &code,
				       const QString &message)
{
	QString detail = message;
	if (code == QStringLiteral("relay_unavailable")) {
		detail = engineStatusText(link_state_, process_state_);
	}

	showError(delaydeck::tr("Error.ControlFailed").arg(code, detail));
}

void DelayDeckDock::applyRequestFailed(const QString &operation,
				       const QString &detail)
{
	if (link_state_ == RelayLinkState::Connected &&
	    operation == QStringLiteral("health")) {
		return;
	}

	showError(delaydeck::tr("Error.RequestFailed")
			  .arg(operationLabel(operation), detail));
}

void DelayDeckDock::updateProcessDisplay()
{
	updateSummaryLabel();
}

void DelayDeckDock::updateSummaryLabel()
{
	const QString engine = engineStatusText(link_state_, process_state_);
	if (link_state_ != RelayLinkState::Connected) {
		summary_label_->setText(engine);
		return;
	}

	QString mode = relayStateLabel(relay_state_);
	if (transition_pending_) {
		mode = delaydeck::tr("Status.TransitionPending").arg(mode);
	}

	if (showsDelayValueForState(relay_state_, active_delay_seconds_)) {
		summary_label_->setText(
			delaydeck::tr("Status.WithDelay")
				.arg(engine, mode,
				     delaydeck::tr("Value.Seconds")
					     .arg(active_delay_seconds_)));
		return;
	}

	summary_label_->setText(delaydeck::tr("Status.Summary").arg(engine, mode));
}

void DelayDeckDock::updateTransitionDisplay(const RelayStatus &status)
{
	if (!showsTransition(status)) {
		transition_label_->hide();
		return;
	}

	const QString text = transitionText(status);
	if (text.isEmpty()) {
		transition_label_->hide();
		return;
	}

	transition_label_->setText(text);
	transition_label_->show();
}

void DelayDeckDock::showError(const QString &text)
{
	error_label_->setText(text);
	error_label_->show();
}

void DelayDeckDock::clearError()
{
	error_label_->clear();
	error_label_->hide();
}

void DelayDeckDock::resetSummaryLabel()
{
	summary_label_->setText(engineStatusText(link_state_, process_state_));
}

void DelayDeckDock::onAdvancedToggled(bool visible)
{
	advanced_panel_->setVisible(visible);
	advanced_toggle_button_->setText(visible
						 ? delaydeck::tr("Section.HideAdvanced")
						 : delaydeck::tr("Section.ShowAdvanced"));
	if (!loading_settings_) {
		scheduleSettingsSave();
	}
}

void DelayDeckDock::scheduleSettingsSave()
{
	if (loading_settings_) {
		return;
	}
	settings_save_timer_.start();
}

void DelayDeckDock::applyDockSettings(int targetDelaySeconds, bool delayStream,
				      bool advancedVisible,
				      const QString &enableSlateScene,
				      const QString &returnSlateScene)
{
	loading_settings_ = true;

	if (targetDelaySeconds >= target_delay_spin_->minimum() &&
	    targetDelaySeconds <= target_delay_spin_->maximum()) {
		target_delay_spin_->setValue(targetDelaySeconds);
	}

	start_with_delay_ = delayStream;
	{
		const QSignalBlocker blocker(delay_toggle_);
		delay_toggle_->setChecked(delayStream);
	}

	{
		const QSignalBlocker blocker(advanced_toggle_button_);
		advanced_toggle_button_->setChecked(advancedVisible);
	}
	onAdvancedToggled(advancedVisible);

	slate_scene_controller_.setEnableSceneName(enableSlateScene);
	slate_scene_controller_.setReturnSceneName(returnSlateScene);
	refreshSceneSelectors();
	selectComboItem(enable_slate_scene_combo_, enableSlateScene);
	selectComboItem(return_slate_scene_combo_, returnSlateScene);
	onEnableSlateSceneChanged();
	onReturnSlateSceneChanged();

	loading_settings_ = false;
	updateButtonStates();
}

void DelayDeckDock::loadSettings(obs_data_t *data)
{
	if (!data) {
		return;
	}

	OBSDataAutoRelease obj = obs_data_get_obj(data, delaydeck::kDockSettingsKey);
	if (!obj) {
		return;
	}

	int targetDelay = static_cast<int>(obs_data_get_int(obj, "target_delay_seconds"));
	if (targetDelay < target_delay_spin_->minimum() ||
	    targetDelay > target_delay_spin_->maximum()) {
		targetDelay = target_delay_spin_->value();
	}

	const bool delayStream = obs_data_get_bool(obj, "delay_stream");
	bool advancedVisible = true;
	if (obs_data_has_user_value(obj, "advanced_visible")) {
		advancedVisible = obs_data_get_bool(obj, "advanced_visible");
	}

	const QString enableScene =
		QString::fromUtf8(obs_data_get_string(obj, "enable_slate_scene"));
	const QString returnScene =
		QString::fromUtf8(obs_data_get_string(obj, "return_slate_scene"));

	applyDockSettings(targetDelay, delayStream, advancedVisible, enableScene,
			  returnScene);
}

void DelayDeckDock::saveSettings(obs_data_t *data) const
{
	if (!data) {
		return;
	}

	OBSDataAutoRelease obj = obs_data_create();
	obs_data_set_int(obj, "target_delay_seconds", target_delay_spin_->value());
	obs_data_set_bool(obj, "delay_stream", start_with_delay_);
	obs_data_set_bool(obj, "advanced_visible", advanced_panel_->isVisible());
	obs_data_set_string(obj, "enable_slate_scene",
			    enable_slate_scene_combo_->currentData()
				    .toString()
				    .toUtf8()
				    .constData());
	obs_data_set_string(obj, "return_slate_scene",
			    return_slate_scene_combo_->currentData()
				    .toString()
				    .toUtf8()
				    .constData());
	obs_data_set_obj(data, delaydeck::kDockSettingsKey, obj);
}

void DelayDeckDock::refreshSceneSelectors()
{
	const QStringList sceneNames = SlateSceneController::sceneNames();
	populateSceneCombo(enable_slate_scene_combo_, sceneNames);
	populateSceneCombo(return_slate_scene_combo_, sceneNames);
	onEnableSlateSceneChanged();
	onReturnSlateSceneChanged();
}

void DelayDeckDock::startRelayClient()
{
	if (relay_process_->isManaged()) {
		if (process_state_ != RelayProcessState::Running) {
			return;
		}
	}
	relay_client_->resumeTraffic();
}

bool DelayDeckDock::canEditDelayTarget() const
{
	if (relay_state_ == QStringLiteral("DELAYED")) {
		return false;
	}
	if (transition_pending_ || isTransitionState(relay_state_)) {
		return false;
	}
	return true;
}

bool DelayDeckDock::canOperateDelayToggle() const
{
	const bool connected = link_state_ == RelayLinkState::Connected;
	const bool processRunning =
		!relay_process_->isManaged() ||
		process_state_ == RelayProcessState::Running;
	if (!connected || !processRunning) {
		return false;
	}

	if (!obs_frontend_streaming_active()) {
		return true;
	}

	if (transition_pending_ || isTransitionState(relay_state_)) {
		return false;
	}

	return relay_state_ == QStringLiteral("REALTIME") ||
	       relay_state_ == QStringLiteral("DELAYED");
}

void DelayDeckDock::syncDelayToggle(const RelayStatus &status)
{
	const bool checked =
		delayToggleCheckedForStatus(status, start_with_delay_);
	const QSignalBlocker blocker(delay_toggle_);
	delay_toggle_->setChecked(checked);
}

void DelayDeckDock::maybeAutoEnableDelay()
{
	if (!start_with_delay_ || auto_delay_triggered_) {
		return;
	}
	if (!obs_frontend_streaming_active()) {
		return;
	}
	if (link_state_ != RelayLinkState::Connected) {
		return;
	}
	if (relay_state_ != QStringLiteral("REALTIME") || transition_pending_) {
		return;
	}

	relay_client_->enableDelay(target_delay_spin_->value());
	auto_delay_triggered_ = true;
}

void DelayDeckDock::updateButtonStates()
{
	const bool connected = link_state_ == RelayLinkState::Connected;
	const bool processRunning =
		!relay_process_->isManaged() ||
		process_state_ == RelayProcessState::Running;
	const bool processBusy =
		relay_process_->isManaged() &&
		(process_state_ == RelayProcessState::Starting ||
		 process_state_ == RelayProcessState::Stopping);

	delay_toggle_->setEnabled(canOperateDelayToggle());
	dump_buffer_button_->setEnabled(connected && processRunning);
	target_delay_spin_->setEnabled(connected && processRunning &&
					canEditDelayTarget());
	restart_relay_button_->setEnabled(relay_process_->isManaged() &&
					  !processBusy);
}

void DelayDeckDock::onDelayToggleChanged(bool checked)
{
	const bool streaming = obs_frontend_streaming_active();
	const bool inTransition =
		transition_pending_ || isTransitionState(relay_state_);
	const bool liveEnable =
		checked && streaming &&
		link_state_ == RelayLinkState::Connected &&
		relay_state_ == QStringLiteral("REALTIME") && !inTransition;
	const bool liveDisable =
		!checked && streaming &&
		link_state_ == RelayLinkState::Connected &&
		relay_state_ == QStringLiteral("DELAYED") && !inTransition;

	QString title;
	QString message;
	if (checked) {
		title = delaydeck::tr("ToggleDelay.Enable.Title");
		message = liveEnable
				  ? delaydeck::tr("ToggleDelay.Enable.LiveMessage")
					    .arg(target_delay_spin_->value())
				  : delaydeck::tr("ToggleDelay.Enable.PreStreamMessage")
					    .arg(target_delay_spin_->value());
	} else {
		title = delaydeck::tr("ToggleDelay.Disable.Title");
		message = liveDisable
				  ? delaydeck::tr("ToggleDelay.Disable.LiveMessage")
				  : delaydeck::tr("ToggleDelay.Disable.PreStreamMessage");
	}

	const auto answer = QMessageBox::question(
		this, title, message, QMessageBox::Yes | QMessageBox::No,
		QMessageBox::No);
	if (answer != QMessageBox::Yes) {
		const QSignalBlocker blocker(delay_toggle_);
		delay_toggle_->setChecked(!checked);
		return;
	}

	start_with_delay_ = checked;
	scheduleSettingsSave();

	if (!streaming) {
		return;
	}
	if (link_state_ != RelayLinkState::Connected) {
		return;
	}
	if (inTransition) {
		return;
	}

	if (checked && relay_state_ == QStringLiteral("REALTIME")) {
		relay_client_->enableDelay(target_delay_spin_->value());
		auto_delay_triggered_ = true;
		return;
	}

	if (!checked && relay_state_ == QStringLiteral("DELAYED")) {
		relay_client_->returnLive();
	}
}

void DelayDeckDock::onDumpBufferClicked()
{
	const auto answer = QMessageBox::question(
		this, delaydeck::tr("DumpBuffer.Title"), delaydeck::tr("DumpBuffer.Message"),
		QMessageBox::Yes | QMessageBox::No, QMessageBox::No);
	if (answer != QMessageBox::Yes) {
		return;
	}

	start_with_delay_ = false;
	{
		const QSignalBlocker blocker(delay_toggle_);
		delay_toggle_->setChecked(false);
	}
	relay_client_->dumpBuffer();
	scheduleSettingsSave();
}

void DelayDeckDock::onRestartRelayClicked()
{
	const auto answer = QMessageBox::question(
		this, delaydeck::tr("RestartRelay.Title"),
		delaydeck::tr("RestartRelay.Message"), QMessageBox::Yes | QMessageBox::No,
		QMessageBox::No);
	if (answer != QMessageBox::Yes) {
		return;
	}

	clearError();
	relay_client_->suspendTraffic();
	relay_process_->restartRelay();
}

void DelayDeckDock::onEnableSlateSceneChanged()
{
	slate_scene_controller_.setEnableSceneName(
		enable_slate_scene_combo_->currentData().toString());
	scheduleSettingsSave();
}

void DelayDeckDock::onReturnSlateSceneChanged()
{
	slate_scene_controller_.setReturnSceneName(
		return_slate_scene_combo_->currentData().toString());
	scheduleSettingsSave();
}
