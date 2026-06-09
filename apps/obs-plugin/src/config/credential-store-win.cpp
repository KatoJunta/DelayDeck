#include "config/credential-store.hpp"

#ifdef _WIN32

#ifndef WINVER
#define WINVER 0x0601
#endif
#ifndef _WIN32_WINNT
#define _WIN32_WINNT 0x0601
#endif
#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>
#include <wincred.h>
#include <wincrypt.h>

#include <obs-module.h>

#include <QDir>
#include <QFile>
#include <QFileInfo>

#include <vector>

namespace delaydeck {

namespace {

constexpr wchar_t kTargetName[] = L"DelayDeck.RelayOutputStreamKey";
constexpr wchar_t kLegacyTargetName[] = L"DelayDeck/RelayOutputStreamKey";
constexpr wchar_t kUserName[] = L"DelayDeck";
constexpr char kDpapiFileName[] = "stream_key.dpapi";

QString dpapiFilePath()
{
	char *path = obs_module_config_path(kDpapiFileName);
	if (!path) {
		return {};
	}

	const QString result = QString::fromUtf8(path);
	bfree(path);
	return result;
}

bool writeDpapiFile(const QByteArray &plain)
{
	DATA_BLOB input = {};
	input.pbData = reinterpret_cast<BYTE *>(const_cast<char *>(plain.data()));
	input.cbData = static_cast<DWORD>(plain.size());

	DATA_BLOB output = {};
	if (!CryptProtectData(&input, L"DelayDeck relay stream key", nullptr,
			      nullptr, nullptr, CRYPTPROTECT_UI_FORBIDDEN,
			      &output)) {
		blog(LOG_WARNING, "[DelayDeck] CryptProtectData failed (%lu)",
		     GetLastError());
		return false;
	}

	const QString path = dpapiFilePath();
	if (path.isEmpty()) {
		LocalFree(output.pbData);
		return false;
	}

	QDir().mkpath(QFileInfo(path).absolutePath());

	QFile file(path);
	if (!file.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
		LocalFree(output.pbData);
		blog(LOG_WARNING, "[DelayDeck] could not write DPAPI store file");
		return false;
	}

	const qint64 written =
		file.write(reinterpret_cast<const char *>(output.pbData),
			   static_cast<qint64>(output.cbData));
	file.close();
	LocalFree(output.pbData);

	return written == static_cast<qint64>(output.cbData);
}

bool readDpapiFile(QByteArray *plain)
{
	if (!plain) {
		return false;
	}

	const QString path = dpapiFilePath();
	if (path.isEmpty() || !QFileInfo::exists(path)) {
		return false;
	}

	QFile file(path);
	if (!file.open(QIODevice::ReadOnly)) {
		return false;
	}

	const QByteArray cipher = file.readAll();
	file.close();
	if (cipher.isEmpty()) {
		return false;
	}

	DATA_BLOB input = {};
	input.pbData = reinterpret_cast<BYTE *>(const_cast<char *>(cipher.data()));
	input.cbData = static_cast<DWORD>(cipher.size());

	DATA_BLOB output = {};
	if (!CryptUnprotectData(&input, nullptr, nullptr, nullptr, nullptr, 0,
				&output)) {
		return false;
	}

	*plain = QByteArray(reinterpret_cast<const char *>(output.pbData),
			    static_cast<qsizetype>(output.cbData));
	LocalFree(output.pbData);
	return !plain->isEmpty();
}

void deleteDpapiFile()
{
	const QString path = dpapiFilePath();
	if (!path.isEmpty()) {
		QFile::remove(path);
	}
}

void deleteCredentialTargets()
{
	CredDeleteW(kTargetName, CRED_TYPE_GENERIC, 0);
	CredDeleteW(kLegacyTargetName, CRED_TYPE_GENERIC, 0);
}

bool writeCredentialManager(const QByteArray &keyBytes)
{
	deleteCredentialTargets();

	std::vector<BYTE> blob(keyBytes.begin(), keyBytes.end());

	CREDENTIALW cred = {};
	cred.Type = CRED_TYPE_GENERIC;
	cred.TargetName = const_cast<LPWSTR>(kTargetName);
	cred.UserName = const_cast<LPWSTR>(kUserName);
	cred.CredentialBlobSize = static_cast<DWORD>(blob.size());
	cred.CredentialBlob = blob.data();
	cred.Persist = CRED_PERSIST_LOCAL_MACHINE;

	if (CredWriteW(&cred, 0) != FALSE) {
		return true;
	}

	blog(LOG_WARNING, "[DelayDeck] CredWrite failed (%lu)", GetLastError());
	return false;
}

} // namespace

bool writeStreamKey(const QString &streamKey)
{
	if (streamKey.trimmed().isEmpty()) {
		return false;
	}

	const QByteArray keyBytes = streamKey.toUtf8();

	if (writeCredentialManager(keyBytes)) {
		deleteDpapiFile();
		return true;
	}

	if (writeDpapiFile(keyBytes)) {
		return true;
	}

	blog(LOG_ERROR,
	     "[DelayDeck] stream key storage failed (Credential Manager and DPAPI)");
	return false;
}

bool readStreamKey(QString *streamKey)
{
	if (!streamKey) {
		return false;
	}

	PCREDENTIALW cred = nullptr;
	if (CredReadW(kTargetName, CRED_TYPE_GENERIC, 0, &cred)) {
		*streamKey =
			QString::fromUtf8(reinterpret_cast<const char *>(
						  cred->CredentialBlob),
					  static_cast<int>(cred->CredentialBlobSize));
		CredFree(cred);
		return !streamKey->isEmpty();
	}

	if (CredReadW(kLegacyTargetName, CRED_TYPE_GENERIC, 0, &cred)) {
		*streamKey =
			QString::fromUtf8(reinterpret_cast<const char *>(
						  cred->CredentialBlob),
					  static_cast<int>(cred->CredentialBlobSize));
		CredFree(cred);
		return !streamKey->isEmpty();
	}

	QByteArray plain;
	if (readDpapiFile(&plain)) {
		*streamKey = QString::fromUtf8(plain);
		return !streamKey->isEmpty();
	}

	return false;
}

void deleteStreamKey()
{
	deleteCredentialTargets();
	deleteDpapiFile();
}

} // namespace delaydeck

#else

namespace delaydeck {

bool writeStreamKey(const QString &)
{
	return false;
}

bool readStreamKey(QString *)
{
	return false;
}

void deleteStreamKey() {}

} // namespace delaydeck

#endif
