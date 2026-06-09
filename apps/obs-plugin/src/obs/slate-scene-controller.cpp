#include "slate-scene-controller.hpp"

#include <obs-frontend-api.h>
#include <obs.h>
#include <util/bmem.h>

namespace {

QString sourceName(obs_source_t *source)
{
	if (!source) {
		return {};
	}
	const char *name = obs_source_get_name(source);
	return name ? QString::fromUtf8(name) : QString();
}

obs_source_t *findSceneByName(const QString &name)
{
	if (name.isEmpty()) {
		return nullptr;
	}

	struct obs_frontend_source_list scenes = {};
	obs_frontend_get_scenes(&scenes);

	obs_source_t *matched = nullptr;
	for (size_t i = 0; i < scenes.sources.num; i++) {
		obs_source_t *scene = scenes.sources.array[i];
		if (sourceName(scene) == name) {
			matched = obs_source_get_ref(scene);
			break;
		}
	}

	obs_frontend_source_list_free(&scenes);
	return matched;
}

} // namespace

void SlateSceneController::setEnableSceneName(const QString &name)
{
	enable_scene_name_ = name;
}

void SlateSceneController::setReturnSceneName(const QString &name)
{
	return_scene_name_ = name;
}

void SlateSceneController::applyStatus(const RelayStatus &status)
{
	const SlateKind kind = kindForStatus(status);
	if (kind == SlateKind::None) {
		restoreIfOwned();
		return;
	}

	switchFor(kind);
}

void SlateSceneController::clear()
{
	active_ = false;
	previous_scene_name_.clear();
	controlled_scene_name_.clear();
}

QStringList SlateSceneController::sceneNames()
{
	QStringList names;

	char **scene_names = obs_frontend_get_scene_names();
	for (char **current = scene_names; current && *current; current++) {
		const QString name = QString::fromUtf8(*current);
		if (!name.isEmpty()) {
			names.push_back(name);
		}
	}
	bfree(scene_names);

	return names;
}

SlateSceneController::SlateKind
SlateSceneController::kindForStatus(const RelayStatus &status)
{
	if (status.slateMessage.isEmpty() || status.countdownSeconds <= 0) {
		return SlateKind::None;
	}
	if (status.slateMessage.startsWith(QStringLiteral("Delay activating"))) {
		return SlateKind::EnableDelay;
	}
	if (status.slateMessage.startsWith(QStringLiteral("Returning to realtime"))) {
		return SlateKind::ReturnLive;
	}
	return SlateKind::None;
}

bool SlateSceneController::switchToScene(const QString &name)
{
	obs_source_t *scene = findSceneByName(name);
	if (!scene) {
		return false;
	}

	obs_frontend_set_current_scene(scene);
	obs_source_release(scene);
	return true;
}

QString SlateSceneController::currentSceneName()
{
	obs_source_t *current = obs_frontend_get_current_scene();
	const QString name = sourceName(current);
	if (current) {
		obs_source_release(current);
	}
	return name;
}

void SlateSceneController::switchFor(SlateKind kind)
{
	const QString target = kind == SlateKind::EnableDelay ? enable_scene_name_
						     : return_scene_name_;
	if (target.isEmpty()) {
		return;
	}

	const QString current = currentSceneName();
	if (active_ && current != controlled_scene_name_) {
		clear();
		return;
	}

	if (!active_) {
		previous_scene_name_ = current;
	}
	if (current == target) {
		active_ = true;
		controlled_scene_name_ = target;
		return;
	}
	if (switchToScene(target)) {
		active_ = true;
		controlled_scene_name_ = target;
	}
}

void SlateSceneController::restoreIfOwned()
{
	if (!active_) {
		return;
	}

	const QString current = currentSceneName();
	if (current == controlled_scene_name_ && !previous_scene_name_.isEmpty()) {
		(void)switchToScene(previous_scene_name_);
	}

	clear();
}
