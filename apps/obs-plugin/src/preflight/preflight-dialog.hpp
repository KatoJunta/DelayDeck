#pragma once

#include "preflight/preflight-result.hpp"

#include <QWidget>

class PreflightDialog final {
public:
	static QString messageFor(const PreflightResult &result);
	static void showFailure(QWidget *parent, const PreflightResult &result);
};
