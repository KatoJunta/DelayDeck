#pragma once

#include "preflight/preflight-result.hpp"
#include "relay/relay-process.hpp"

#include <QString>

class PreflightChecker final {
public:
	static PreflightResult run(RelayProcessState processState, bool managedRelay,
				   const QString &apiBaseUrl,
				   const QString &sessionToken);

private:
	static bool delayDeckModeEnabled();
	static bool ingestEndpoint(QString *host, quint16 *port);
	static bool isLoopbackHost(const QString &host);
	static bool probeTcpEndpoint(const QString &host, quint16 port,
				     int timeoutMs);
	static bool fetchRelayHealthy(const QString &apiBaseUrl, QString *detail);
	static PreflightResult checkObsDestination(const QString &expectedHost,
						     quint16 expectedPort);
	static PreflightResult checkObsNativeStreamDelay();
};
