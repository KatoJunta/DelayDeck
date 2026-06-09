#pragma once

#include "relay/relay-types.hpp"

#include <QString>
#include <QStringList>

class SlateSceneController final {
public:
	void setEnableSceneName(const QString &name);
	void setReturnSceneName(const QString &name);

	void applyStatus(const RelayStatus &status);
	void clear();

	static QStringList sceneNames();
	static bool sceneExists(const QString &name);

private:
	enum class SlateKind {
		None,
		EnableDelay,
		ReturnLive,
	};

	static SlateKind kindForStatus(const RelayStatus &status);
	static bool switchToScene(const QString &name);
	static QString currentSceneName();

	void switchFor(SlateKind kind);
	void restoreIfOwned();

	QString enable_scene_name_;
	QString return_scene_name_;
	QString previous_scene_name_;
	QString controlled_scene_name_;
	bool active_ = false;
};
