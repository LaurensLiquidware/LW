# ProfileUnity MSP Licensing Console — User Manual

This is an operator manual: how to install, configure, and use the
console day to day. For build status, architecture, and compliance
details, see [`README.md`](../README.md); for the version-by-version
history of what was added, see [`CHANGELOG.md`](../CHANGELOG.md).

> **This is a Liquidware Sparks Tool** — a community/field-contributed
> utility, not a Liquidware commercial product, provided "AS IS" with no
> warranty, support, or maintenance. See `Spark_License.pdf` and the
> About screen inside the console.

## Contents

- [What this does](#what-this-does)
- [Installing and starting the server](#installing-and-starting-the-server)
- [Configuration reference](#configuration-reference)
- [Settings screen](#settings-screen)
- [First sign-in](#first-sign-in)
- [Managing tenants](#managing-tenants)
- [Dashboard](#dashboard)
- [Alerts](#alerts)
- [History](#history)
- [Monthly reports](#monthly-reports)
- [About screen](#about-screen)
- [Understanding the status language](#understanding-the-status-language)
- [Troubleshooting](#troubleshooting)

## What this does

The console lets a Managed Service Provider see ProfileUnity license
consumption across every customer console it manages, track that usage
over time, and produce monthly reports — all from one place, without
logging into each customer's ProfileUnity console individually.

The single most important thing to understand before using it:
**ProfileUnity itself keeps no usage history** — its API only ever
returns a point-in-time snapshot. This console builds that history
itself, one collection at a time, starting from the day you register
each tenant. There is no way to backfill data from before that. A day
the scheduler couldn't reach a console is lost data for that tenant on
that day — it is never silently treated as "zero usage," and every
screen in the console goes out of its way to show that day as *unknown*,
not as an interpolated guess.

## Installing and starting the server

The console ships as a self-contained server binary
(`profileunity-msp-console-server` on Windows, `profileunity-msp-console`
on Linux) with the web frontend, license text, and SBOM built in —
nothing extra to install alongside it besides a place to store its
SQLite database file. The Windows zip also ships
`profileunity-msp-console.exe`, a small tray launcher — see "Starting on
Windows" below.

1. Copy `.env.example` to `.env`, in the same folder you'll run the
   binary from, and adjust any values that matter for your deployment
   (see the reference below) — everything has a sensible zero-config
   default, so this step is optional if the defaults suit you. This is a
   one-time step — after that, just starting the binary from that folder
   picks the file up automatically, with no environment variables to set
   by hand each time. (If you'd rather set real environment variables
   instead — e.g. in a Windows Service definition — that works too, and
   takes priority over anything in `.env`.)
2. Run the binary (or `make run` from source). On first startup it will:
   - create the SQLite database file (and run migrations) if it doesn't
     exist yet,
   - generate a self-signed TLS certificate at the configured paths if
     none exists there already (the server only ever serves HTTPS — there
     is no plain-HTTP mode),
   - generate a tenant-credential encryption key if none was supplied
     (see `PUMC_CREDENTIAL_ENCRYPTION_KEY`/`_FILE` below — **back this
     file up**, since losing or replacing it makes every previously
     stored tenant credential permanently undecryptable),
   - create the first operator account — either from
     `PUMC_BOOTSTRAP_ADMIN_USERNAME`/`PASSWORD` if you set those, or a
     built-in `LiquidwareMSP`/`LiquidwareMSP` account otherwise (change
     that password from the account/change-password screen as soon as
     you sign in — it's a fixed, publicly-known default, not a secret).
3. Browse to `https://<host>:<port>` (default `https://0.0.0.0:8443` —
   browse to the machine's actual hostname/IP, not literally `0.0.0.0`).
   Your browser will warn about the
   self-signed certificate unless you've replaced it with a real one —
   either by uploading it from the Settings screen after signing in (no
   restart needed), or by dropping a CA-signed pair at the configured
   `PUMC_TLS_CERT_FILE`/`PUMC_TLS_KEY_FILE` paths before the very first
   startup. The self-signed warning is expected on a fresh install and
   safe to accept for internal use, but replace the certificate before
   exposing this beyond a trusted network.

### Starting on Windows

The Windows zip contains two executables:

- **`profileunity-msp-console.exe`** — a small tray launcher. This is
  what you double-click. It starts `profileunity-msp-console-server.exe`
  as a background process, shows Start/Stop/Restart buttons, a status
  indicator, and a clickable link straight to the console in your
  browser, and collapses to a system tray icon while running —
  click the tray icon (or the window's [x] button) to hide it, right-click
  the tray icon for a menu (Show, Start, Stop, Restart, Show Log, Exit).
  **Show Log** opens a window that live-tails the server's log file as
  new lines are written, so you can watch what's happening without
  opening the log file yourself. **Exit** (from the tray menu — not the
  window's [x] button, which just hides it) is what actually stops the
  server and closes the launcher.
- **`profileunity-msp-console-server.exe`** — the actual server, unchanged
  by any of the above. Run this directly instead of the launcher if
  you're starting the console from a script, a Scheduled Task, or a
  Windows Service — it behaves exactly like the Linux binary, logging to
  the console and to its log file with no GUI involved.

Both read `.env`/write their data files (database, TLS cert, log,
encryption key) from whichever folder they're run from, so keep them in
the same folder together with your `.env`.

## Configuration reference

A handful of settings are environment variables the process needs before
it can even open its own database — these stay env-var-only, read once
at startup, and can't be changed from the UI:

| Variable | Purpose | Default |
|---|---|---|
| `PUMC_HTTP_ADDR` | Address the server binds to | `0.0.0.0:8443` (all interfaces) |
| `PUMC_ENVIRONMENT` | `development` or `production` — sets `PUMC_LOG_LEVEL`'s default (see below) | `development` |
| `PUMC_DB_DRIVER` | `sqlite` or `postgres` | `sqlite` |
| `PUMC_DB_DSN` | File path (sqlite) or connection string (postgres) | `./profileunity-msp-console.db` |
| `PUMC_LOG_LEVEL` | `debug`, `info`, `warn`, or `error`. Leave unset to default from `PUMC_ENVIRONMENT` (`debug` in development, `info` otherwise); set explicitly to override that default either way. | *(from `PUMC_ENVIRONMENT`)* |
| `PUMC_LOG_FILE` | Where logs are written, in addition to stderr/console (appended to, not rotated) | `./profileunity-msp-console.log` |
| `PUMC_CREDENTIAL_ENCRYPTION_KEY` | Base64 32-byte (AES-256) key encrypting stored tenant credentials at rest. Leave unset and one is generated automatically on first boot and saved to `PUMC_CREDENTIAL_ENCRYPTION_KEY_FILE` — **back up that file**. Set this explicitly instead to supply/rotate your own key (`openssl rand -base64 32`). | *(auto-generated)* |
| `PUMC_CREDENTIAL_ENCRYPTION_KEY_FILE` | Where the auto-generated key above is saved when `PUMC_CREDENTIAL_ENCRYPTION_KEY` is left unset | `./credential-encryption.key` |
| `PUMC_BOOTSTRAP_ADMIN_USERNAME` / `PASSWORD` | Creates the first operator account at startup, only while the users table is empty. Leave both unset and a built-in `LiquidwareMSP`/`LiquidwareMSP` account is created instead — change that password from the account/change-password screen right after first sign-in. Set both to use your own username/password from the first run instead. | *(`LiquidwareMSP`/`LiquidwareMSP`)* |

Everything else below is a **seed value only**: read from the
environment the very first time the server starts against a fresh
database, then never looked at again — from that point on the database
is the source of truth, and the Settings screen (see below) is how you
change any of it, live, with no restart. Setting these in `.env` after
the first startup has no effect; use the Settings screen instead.

| Variable | Purpose | Default |
|---|---|---|
| `PUMC_TLS_CERT_FILE` / `PUMC_TLS_KEY_FILE` | Where a self-signed certificate is generated on a completely fresh install, before there's anything in the Settings screen yet. Irrelevant after that — see "Settings screen" below. | `./tls-cert.pem` / `./tls-key.pem` |
| `PUMC_COLLECTION_INTERVAL` | How often the scheduler checks whether it's time to collect (Go duration syntax, e.g. `1h`) | `1h` |
| `PUMC_COLLECTION_TIMEZONE` | IANA timezone used to compute the collection-day boundary. Stored timestamps are always UTC regardless. | `UTC` |
| `PUMC_COLLECTION_CONCURRENCY` | Max tenants polled at once | `5` |
| `PUMC_COLLECTION_TENANT_TIMEOUT` | Per-tenant timeout including retries — one dead tenant can never stall the whole run | `30s` |
| `PUMC_SESSION_IDLE_TIMEOUT` | Operator session idle timeout | `30m` |
| `PUMC_SESSION_ABSOLUTE_TIMEOUT` | Hard cap on a session regardless of activity | `12h` |
| `PUMC_SMTP_HOST` | SMTP server for automatic monthly report emails. Leave unset to disable the feature entirely — see "Automatic monthly report emails" below. | *(unset — disabled)* |
| `PUMC_SMTP_PORT` | SMTP port | `587` |
| `PUMC_SMTP_USERNAME` / `PASSWORD` | SMTP credentials. Leave both unset for a relay that doesn't require auth. | *(unset)* |
| `PUMC_SMTP_FROM` | From address on the emailed report. Required once `PUMC_SMTP_HOST` is set. | *(unset)* |
| `PUMC_SMTP_SECURITY` | `starttls`, `tls`, or `none` | `starttls` |
| `PUMC_REPORT_RECIPIENTS` | Comma-separated recipient list for the monthly portfolio report. Required once `PUMC_SMTP_HOST` is set. | *(unset)* |
| `PUMC_REPORT_EMAIL_DAY` | Day of the month (1-28, in `PUMC_COLLECTION_TIMEZONE`) the previous month's report is sent | `1` |

Losing `PUMC_CREDENTIAL_ENCRYPTION_KEY` means losing the ability to
decrypt any tenant credential already stored — keep it somewhere durable
and separate from the database itself.

## Settings screen

Everything in the second table above — SMTP/report-email, collection
tunables, and operator session timeouts — can be changed from **Settings**
in the left nav, by anyone signed in, without editing `.env` or
restarting the process. A save takes effect immediately: the collection
scheduler picks up a new interval or concurrency on its very next tick,
a changed idle timeout applies to sessions that are already logged in,
and enabling SMTP for the first time means the very next monthly check
can send.

The Settings screen also has a **Send Test Email** button next to the
SMTP fields — it sends using whatever is currently typed into the form,
not what's saved, so you can confirm a relay works before committing to
it.

The **TLS Certificate** section shows the currently active certificate
(subject, expiry, and whether it's self-signed) and lets you upload a
real one: paste or choose a PEM certificate and a matching PEM private
key, and it's validated, hot-swapped into the running HTTPS listener,
and saved — no restart, and no window where the server is unreachable.
An invalid or mismatched pair is rejected before anything changes.

## First sign-in

Sign in with the bootstrap operator account (or whichever account an
administrator created for you). The header lets you switch the UI
language (English/Dutch) at any time, without reloading the page — your
choice isn't remembered across a full page reload/reboot yet, so you'll
land back in English if you close and reopen the browser.

**Changing your password.** Click your username in the header (the key
icon next to it) to open the Change Password dialog. You'll need your
current password and a new one at least 12 characters long. There is no
administrator override or password-reset flow — if you forget your
password entirely, an administrator has to reset the account by other
means (see the Troubleshooting section).

## Managing users

**Users** lists every account that can sign in, and lets you add or
remove one. There is no separate "administrator" role — every signed-in
operator can add and remove accounts, the same flat permission model
this console already uses everywhere else.

- **Add User** takes a username and a password (at least 12 characters,
  same rule as Change Password) and creates the account immediately —
  it can sign in right away with that password.
- There is no edit or password reset from this screen: an account only
  ever changes its own password, from Change Password in the header.
  If someone is locked out, another operator can delete their account
  here and create them a fresh one — this is the closest thing to an
  administrator password reset this console has.
- **Delete** removes an account and immediately ends any of its active
  sessions — it can no longer sign in, and if it's currently signed in
  elsewhere, that session stops working right away. You can't delete
  your own account (to avoid locking yourself out), and you can't
  delete the last remaining account (the console would become
  impossible to sign into, with no bootstrap path to recover from that
  short of the database-reset procedure in Troubleshooting).

## Managing tenants

**Tenants** is where you register each customer's ProfileUnity console.

- **Add Tenant** opens a form for display name, hostname, port,
  optional username/password (for a ProfileUnity console that requires
  authentication), TLS certificate verification (on by default — only
  turn off "Skip TLS Certificate Verification" for consoles you trust on
  a private network; this is a deliberate compromise, never a default),
  enabled/disabled, comma-separated tags, and free-text notes.
- **Test Connection** in the form reports exactly what happened, never
  just pass/fail: unauthenticated success, authenticated success, a TLS
  failure, a timeout, the console rejecting the credentials, the console
  requiring credentials you didn't supply, a response that didn't parse
  as ProfileUnity's expected format, or the console being unreachable
  outright. Use this before saving a new tenant, and any time collection
  starts failing, to narrow down why.
- **Disabling** a tenant (the "Enabled" toggle) stops the scheduler from
  polling it without deleting its history — use this for a customer
  that's temporarily offline or being decommissioned, rather than
  deleting the tenant outright.
- **Deleting** a tenant is permanent and also deletes its entire
  collection history. There is a confirmation prompt; there is no undo.
- A stored password is never shown again once saved, even masked —
  leave the password field blank when editing a tenant to keep the
  currently-stored password unchanged.

## Dashboard

The Dashboard is the at-a-glance view: one row per tenant, sortable and
filterable, showing:

- **Usage**: Good/Fair/Poor, from used ÷ total licenses on the most
  recent *successful* collection.
- **Utilization**: the actual percentage behind that Usage badge. Can
  read over 100% — a tenant genuinely over its license limit shows
  exactly that, not a capped bar.
- **Expiry**: OK / Expiring Soon / Expired, from the console's reported
  support end date.
- **Days Left**: the exact day count behind that Expiry badge (negative
  once expired).
- **Data**: whether the figures above can currently be trusted — see
  [Understanding the status language](#understanding-the-status-language)
  below; this is the single most important column on the screen.
- **License Mode**, **Product**, **Console Version**, **Last Successful
  Collection**: reported as-is from the tenant's most recent successful
  poll. Product is ProfileUnity's own license product name (e.g.
  "ProU+FlexApp") — useful when an MSP's tenants are licensed under
  different ProfileUnity products.

Use the search box to filter by display name, hostname, license mode, or
license product; click a column header to sort by it.

**Collect Now.** Ordinarily the console polls every enabled tenant on a
fixed schedule (`PUMC_COLLECTION_INTERVAL`, default hourly). Click
**Collect Now** in the Dashboard's toolbar to trigger that same poll
immediately instead of waiting for the next tick — useful right after
adding a tenant, since otherwise it shows **Never Collected** until the
schedule catches up. It polls every enabled tenant, blocks until the run
finishes, and refreshes the table with the result.

## Alerts

The bell icon in the header shows a count badge whenever at least one
tenant needs attention, and a popover listing which tenants and why.
A tenant is alertable when any of the following is true (a tenant can
have more than one reason at once, and all of them are shown):

- **Usage is at or over the license limit** ("poor" usage) right now.
- **Support has expired, or is expiring within 30 days.**
- **Data can't currently be trusted** — the tenant's last collection
  attempt failed, its last success is more than 2 days old, or it has
  never been collected at all. Treat this one as the most urgent of the
  three: it means the other two columns for that tenant might already be
  wrong, and nobody would know from looking at them alone.

Alerts are in-app only — there is no email/SMTP notification. Check the
bell whenever you're in the console; it refreshes every time you
navigate to a different screen.

## History

The History screen plots a tenant's (or the whole portfolio's) used vs.
entitled licenses over time as a line chart, plus a list of exactly when
entitlement changed (e.g. a customer's contract was renewed at a higher
seat count).

**Read the gaps as gaps.** A day with no successful collection — whether
it was never attempted, failed outright, or the console was
unreachable — shows as a visible break in the line, never as an
interpolated slope and never as a drop to zero. If a chart shows a gap,
that means *the console doesn't know* what usage was that day, not that
usage was zero.

Switch between **Per Tenant** (pick a tenant from the dropdown) and
**Portfolio** (summed across every registered tenant) with the toggle in
the top right.

## Monthly reports

The Reports screen produces a usage/entitlement summary for one
tenant, or the whole portfolio, for a chosen month — either viewed
on-screen or downloaded as a PDF (the "Download PDF" link; the PDF
contains the identical figures, so the two are always consistent with
each other). The PDF carries the same Liquidware branding as the
console itself — a blue header band with the logo, and a footer with
page numbers — on every page, including multi-tenant portfolio reports
that run to several pages.

Every report leads with an explicit **coverage** badge:

- **Complete** — every day in the month has a successful collection;
  the figures below are as accurate as this console can make them.
- **Partial** — some days in the month were missed or failed; the
  figures are still computed from whatever successful days exist, but
  treat them as a lower bound, not a precise total.
- **No Data** — there wasn't a single successful collection all month;
  the numeric figures shown alongside this badge aren't meaningful.

Below the coverage badge: days collected vs. days in the month, peak and
average used-license count, entitled-license count as of month end, the
ProfileUnity license product as of month end, and a list of any
entitlement changes detected during that month. The portfolio view
additionally breaks down coverage and figures per tenant in a table, so
you can see at a glance which customers had a clean month and which
didn't.

### Automatic monthly report emails

Set the SMTP fields (host, from address, recipients — either at
bootstrap via `PUMC_SMTP_*`/`PUMC_REPORT_RECIPIENTS`, or afterward from
the Settings screen) and the console emails the same portfolio PDF you'd
get from the Reports screen's "Download PDF" link, automatically, once a
month — no separate cron job or external scheduler needed. By default it
sends on the 1st of each month, covering the month that just ended (the
Settings screen's "Send Day of Month" changes the day). Each month is
sent at most once: the server tracks which months it has already
emailed, so a restart or a missed tick never causes a duplicate send,
and if the server is down on the send day it emails as soon as it's back
up and notices the month is still unsent. An empty SMTP host disables
the feature entirely — nothing is scheduled or sent — and turning it on
from the Settings screen takes effect on the very next check, no
restart.

The Settings screen's **Send Now** button sends that same last-month
report immediately, without waiting for the send day — useful to verify
SMTP and recipients are actually working, or to send the report early.
It asks for confirmation first, since (unlike Send Test Email) it emails
every configured recipient, not just an address typed in for a test, and
it counts as that month's send — the automatic scheduled send won't
duplicate it later in the month.

## About screen

Reachable from the nav — opens as a popup rather than navigating away,
so there's nothing to "go back" from. (It's also reachable at `/about`
without signing in, for anyone who needs to check compliance details
before deploying this tool — that version is a full page since there's
no console screen behind it to pop up over; the login screen itself no
longer links to it, but the URL still works.) Shows the running
version, who built it, the required Sparks Tool disclaimer, a link to
`Spark_License.pdf` (the Sparks Tool License and Disclaimer), and a link
to `bom.cdx.json` — a CycloneDX 1.6 Software Bill of Materials listing
every third-party component this build includes, for your security team
to review against your own policy. `THIRD-PARTY-NOTICES.txt` (shipped
alongside the binary, not linked from the UI) lists the license each of
those components is under.

## Understanding the status language

The console deliberately uses two different visual languages, and never
mixes them:

- **Good / Fair / Poor** (green / amber / red) describes usage and
  expiry — how a tenant is doing against its own limits.
- **Neutral gray, with a distinct icon** describes whether the data
  itself can be trusted: **Current**, **Stale**, **Collection Failing**,
  **Never Collected** (dashboard), or **Complete / Partial / No Data**
  (monthly reports). These never borrow the red/amber/green palette.

This is intentional, not a style inconsistency: a console that's
unreachable, or whose data is a week stale, is a completely different
problem from a console that's genuinely over its license limit — and a
"data" state rendered in red would look exactly like a "poor" state at a
glance, hiding that difference at precisely the moment it matters most.
If you ever see a tenant with a gray/neutral data badge, treat every
other column for that tenant as unverified until that badge turns green
again.

## Troubleshooting

**Browser warns about an untrusted certificate.** Expected until you
replace the self-signed one with a CA-signed pair — upload it from the
Settings screen (takes effect immediately, no restart) or drop the files
at `PUMC_TLS_CERT_FILE`/`PUMC_TLS_KEY_FILE` before a fresh install's
first startup. Safe to accept for internal/trusted-network use in the
meantime.

**A tenant shows "Never Collected" and never changes.** Check Test
Connection on that tenant first — it will tell you exactly what's wrong
(unreachable, TLS failure, timeout, credentials rejected/required,
malformed response) rather than a bare failure. Also confirm the tenant
is Enabled — a disabled tenant is never polled by the scheduler.

**A tenant that was fine yesterday now shows "Collection Failing."**
Same first step: Test Connection. Common causes are the customer's
console being temporarily down, a changed password not yet updated here,
or a certificate that changed on their end (if TLS verification is on).

**I need to know the scheduler's own health, not a specific tenant's.**
`GET /healthz` reports overall process status, version, and the
scheduler's last-run outcome — useful for external monitoring or a quick
sanity check that collection is actually running at all.

**I switched languages and it reverted to English.** Expected for now —
the language choice is a runtime UI setting, not yet persisted across a
full page reload. Switch it again after reloading.

**Nobody can sign in.** A fresh install always gets a first account —
either `PUMC_BOOTSTRAP_ADMIN_USERNAME`/`PASSWORD` if you set those, or
the built-in `LiquidwareMSP`/`LiquidwareMSP` default otherwise (the
server logs which one it created at startup). If truly nobody can sign
in, the users table must already be non-empty with credentials nobody
has — recovery is via direct database access, since bootstrap only ever
fires once, when the users table is empty; it will never overwrite or
reset an existing account.

**I forgot my password.** There is no self-service or administrator
password reset. If you're still signed in somewhere, use Change
Password (click your username in the header) instead of getting locked
out. If another operator account exists and is still signed in, they
can delete your locked-out account from the Users screen and create you
a new one — the closest thing to an administrator reset this console
has. If you're already locked out and this is a test/lab instance with
nothing worth keeping, the only way back in is to delete the SQLite
database file (`PUMC_DB_DSN`, default `./profileunity-msp-console.db`)
and restart — this re-triggers the bootstrap above using whatever
`PUMC_BOOTSTRAP_ADMIN_USERNAME`/`PASSWORD` are set, but it also erases
every tenant and every collected snapshot. There is currently no way to
reset just the password without losing that data.
