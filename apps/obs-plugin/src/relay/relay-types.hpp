#pragma once

#include <QJsonObject>
#include <QString>

struct RelayHealth {
	bool healthy = false;
	QString version;
	QString mode;
	QString uptimeSeconds;
};

struct RelayStatus {
	QString state;
	int targetDelaySeconds = 0;
	int activeDelaySeconds = 0;
	double bufferUsagePercent = 0.0;
	bool inputConnected = false;
	bool outputConnected = false;
	QString inputState;
	QString outputState;
	bool transitionPending = false;
	QString slateMessage;
	int countdownSeconds = 0;
	QString lastError;
};

struct RelayControlError {
	QString code;
	QString message;
};

inline RelayHealth parseHealth(const QJsonObject &obj)
{
	RelayHealth health;
	health.healthy = obj.value(QStringLiteral("healthy")).toBool();
	health.version = obj.value(QStringLiteral("version")).toString();
	health.mode = obj.value(QStringLiteral("mode")).toString();
	health.uptimeSeconds = obj.value(QStringLiteral("uptime_seconds")).toString();
	return health;
}

inline RelayStatus parseStatus(const QJsonObject &obj)
{
	RelayStatus status;
	status.state = obj.value(QStringLiteral("state")).toString();
	status.targetDelaySeconds =
		obj.value(QStringLiteral("target_delay_seconds")).toInt();
	status.activeDelaySeconds =
		obj.value(QStringLiteral("active_delay_seconds")).toInt();
	status.bufferUsagePercent =
		obj.value(QStringLiteral("buffer_usage_percent")).toDouble();
	status.inputConnected =
		obj.value(QStringLiteral("input_connected")).toBool();
	status.outputConnected =
		obj.value(QStringLiteral("output_connected")).toBool();
	status.inputState = obj.value(QStringLiteral("input_state")).toString();
	status.outputState = obj.value(QStringLiteral("output_state")).toString();
	status.transitionPending =
		obj.value(QStringLiteral("transition_pending")).toBool();
	status.slateMessage = obj.value(QStringLiteral("slate_message")).toString();
	status.countdownSeconds =
		obj.value(QStringLiteral("countdown_seconds")).toInt();
	status.lastError = obj.value(QStringLiteral("last_error")).toString();
	return status;
}

inline bool parseControlError(const QJsonObject &obj, RelayControlError *out)
{
	const auto error = obj.value(QStringLiteral("error")).toObject();
	if (error.isEmpty()) {
		return false;
	}
	out->code = error.value(QStringLiteral("code")).toString();
	out->message = error.value(QStringLiteral("message")).toString();
	return !out->code.isEmpty();
}
