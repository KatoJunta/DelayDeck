#include "preflight-dialog.hpp"

#include "locale/tr.hpp"

#include <QMessageBox>

QString PreflightDialog::messageFor(const PreflightResult &result)
{
	switch (result.code) {
	case PreflightFailureCode::RelayProcessNotRunning:
		return delaydeck::tr("Preflight.RelayProcessNotRunning");
	case PreflightFailureCode::RelayUnreachable:
		return delaydeck::tr("Preflight.RelayUnreachable").arg(result.detail);
	case PreflightFailureCode::RelayUnhealthy:
		return delaydeck::tr("Preflight.RelayUnhealthy").arg(result.detail);
	case PreflightFailureCode::IngestNotListening:
		return delaydeck::tr("Preflight.IngestNotListening").arg(result.detail);
	case PreflightFailureCode::DestinationNotConfigured:
		return delaydeck::tr("Preflight.DestinationNotConfigured");
	case PreflightFailureCode::DestinationNotLocalRelay:
		return delaydeck::tr("Preflight.DestinationNotLocalRelay").arg(result.detail);
	case PreflightFailureCode::None:
		break;
	}
	return delaydeck::tr("Preflight.Unknown");
}

void PreflightDialog::showFailure(QWidget *parent, const PreflightResult &result)
{
	QMessageBox box(parent);
	box.setIcon(QMessageBox::Warning);
	box.setWindowTitle(delaydeck::tr("Preflight.Title"));
	box.setText(messageFor(result));
	box.setStandardButtons(QMessageBox::Ok);
	box.exec();
}
