#include "preflight-dialog.hpp"

#include "locale/tr.hpp"

#include <QMessageBox>

QString PreflightDialog::messageFor(const PreflightResult &result)
{
	switch (result.code) {
	case PreflightFailureCode::RelayProcessNotRunning:
		return tr("Preflight.RelayProcessNotRunning");
	case PreflightFailureCode::RelayUnreachable:
		return tr("Preflight.RelayUnreachable").arg(result.detail);
	case PreflightFailureCode::RelayUnhealthy:
		return tr("Preflight.RelayUnhealthy").arg(result.detail);
	case PreflightFailureCode::IngestNotListening:
		return tr("Preflight.IngestNotListening").arg(result.detail);
	case PreflightFailureCode::DestinationNotConfigured:
		return tr("Preflight.DestinationNotConfigured");
	case PreflightFailureCode::DestinationNotLocalRelay:
		return tr("Preflight.DestinationNotLocalRelay").arg(result.detail);
	case PreflightFailureCode::None:
		break;
	}
	return tr("Preflight.Unknown");
}

void PreflightDialog::showFailure(QWidget *parent, const PreflightResult &result)
{
	QMessageBox box(parent);
	box.setIcon(QMessageBox::Warning);
	box.setWindowTitle(tr("Preflight.Title"));
	box.setText(messageFor(result));
	box.setStandardButtons(QMessageBox::Ok);
	box.exec();
}
