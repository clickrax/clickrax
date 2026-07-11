# Security Policy / Политика безопасности

[English](#english) · [Русский](#русский)

---

<a id="english"></a>

## English

### Supported versions

| Version | Supported |
|---------|-----------|
| 2.3.x   | Yes       |
| < 2.3   | No        |

### Reporting a vulnerability

Please **do not** open public GitHub Issues for security-sensitive reports.

1. Use [GitHub Security Advisories](https://github.com/clickrax/clickrax/security/advisories/new) (private vulnerability reporting), if enabled.
2. Or write privately on Telegram: [@Johnwatson7777](https://t.me/Johnwatson7777).

We aim to acknowledge reports within 7 days.

### Threat model

ClickRAX is a **Windows backup client**. It writes backups to destinations you configure (Proxmox Backup Server, SMB, FTP/FTPS).

**What the client protects**

- PBS tokens, SMB/FTP passwords, SMTP passwords, and encryption passphrases are **not stored in `config.json`**. They live in Windows DPAPI files and Credential Manager.
- `config.json` is protected with HMAC integrity (`config.json.hmac`) and restrictive ACLs.
- PBS connections use HTTPS; optional certificate fingerprint pinning.
- Webhook URLs are validated (HTTPS only, no private/loopback targets).

**What the client does not protect against**

- **Host compromise (ransomware, malware, stolen admin session):** an attacker with sufficient privileges can read DPAPI blobs, steal destination credentials, and **delete or overwrite remote backups** — without using ClickRAX.
- **Same-site SMB backups:** immutability is not provided by the client.
- **Privileged admin tampering:** an administrator can modify config and re-sign the HMAC.

ClickRAX is **not** an immutable backup appliance. For ransomware resilience, use **off-host destinations**, **least-privilege PBS tokens**, and **append-only or immutable storage**.

### Recommendations for administrators

1. **PBS token:** grant only `DatastoreBackup` and `DatastoreAudit`; avoid prune/delete on production tokens.
2. **SMB:** prefer a remote share; use append-only ACLs where possible.
3. **Service:** Windows service runs as LocalSystem by default — protect the host and `%ProgramData%\ClickRAX\`.
4. **Updates:** install releases from [GitHub Releases](https://github.com/clickrax/clickrax/releases) (first builds are unsigned; see [code signing policy](docs/code-signing-policy.md)).

### Development

Never commit `config.json`, `*.dpapi`, real PBS URLs, tokens, or internal IPs.

See also [docs/privacy.md](docs/privacy.md) and [docs/code-signing-policy.md](docs/code-signing-policy.md).

---

<a id="русский"></a>

## Русский

### Поддерживаемые версии

| Версия | Поддержка |
|--------|-----------|
| 2.3.x  | Да        |
| < 2.3  | Нет       |

### Сообщение об уязвимости

**Не** создавайте публичные Issues для чувствительных отчётов о безопасности.

1. [GitHub Security Advisories](https://github.com/clickrax/clickrax/security/advisories/new) (приватное сообщение), если включено.
2. Или напишите в Telegram: [@Johnwatson7777](https://t.me/Johnwatson7777).

Подтверждение — в течение 7 дней.

### Модель угроз

ClickRAX — **клиент резервного копирования Windows**. Пишет бэкапы на назначения, которые вы настраиваете (PBS, SMB, FTP/FTPS).

**Что клиент защищает**

- Токены PBS, пароли SMB/FTP/SMTP и passphrase шифрования **не хранятся в `config.json`** — только в DPAPI и Credential Manager.
- `config.json` защищён HMAC (`config.json.hmac`) и ACL.
- PBS — только HTTPS; опциональная привязка к fingerprint сертификата.
- Webhook — только HTTPS, без loopback/private IP.

**От чего клиент не защищает**

- **Захват хоста (ransomware, malware):** атакующий с достаточными правами может прочитать DPAPI, украсть креды и **удалить удалённые бэкапы** напрямую.
- **SMB на том же хосте:** immutability клиент не обеспечивает.
- **Админ с правами:** может изменить config и переподписать HMAC.

ClickRAX — **не** immutable backup appliance. Для устойчивости к ransomware: **off-host хранилище**, **least-privilege PBS-токены**, **append-only/immutable SMB**.

### Рекомендации администраторам

1. **PBS-токен:** только `DatastoreBackup` + `DatastoreAudit`; без prune/delete на production.
2. **SMB:** удалённый share; append-only ACL где возможно.
3. **Служба:** по умолчанию LocalSystem — защищайте хост и `%ProgramData%\ClickRAX\`.
4. **Обновления:** релизы с [GitHub Releases](https://github.com/clickrax/clickrax/releases) (первые сборки без подписи; см. [политику подписи](docs/code-signing-policy.md)).

### Разработка

Не коммитьте `config.json`, `*.dpapi`, реальные URL PBS, токены и внутренние IP.

См. также [docs/privacy.md](docs/privacy.md) и [docs/code-signing-policy.md](docs/code-signing-policy.md).
