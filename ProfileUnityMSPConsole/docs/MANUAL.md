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

The console ships as a single self-contained binary (`profileunity-msp-console`)
with the web frontend, license text, and SBOM built in — nothing extra
to install alongside it besides a place to store its SQLite database
file.

1. Copy `.env.example` to `.env`, in the same folder you'll run the
   binary from, and fill in the values that matter for your deployment
   (see the reference below). At minimum you need `PUMC_HTTP_ADDR` set —
   there is no default listen address, since this is meant to run for a
   team, not just `localhost`. This is a one-time step — after that, just
   starting the binary from that folder picks the file up automatically,
   with no environment variables to set by hand each time. (If you'd
   rather set real environment variables instead — e.g. in a Windows
   Service definition — that works too, and takes priority over anything
   in `.env`.)
2. Run the binary (or `make run` from source). On first startup it will:
   - create the SQLite database file (and run migrations) if it doesn't
     exist yet,
   - generate a self-signed TLS certificate at the configured paths if
     none exists there already (the server only ever serves HTTPS — there
     is no plain-HTTP mode),
   - create the first operator account from
     `PUMC_BOOTSTRAP_ADMIN_USERNAME`/`PASSWORD`, if the users table is
     empty and those are set.
3. Browse to `https://<host>:<port>`. Your browser will warn about the
   self-signed certificate unless you've replaced it with a CA-signed
   pair at the configured `PUMC_TLS_CERT_FILE`/`PUMC_TLS_KEY_FILE` paths —
   that warning is expected on a fresh install and safe to accept for
   internal use, but replace the certificate before exposing this beyond
   a trusted network.

## Configuration reference

All configuration is environment variables, read once at startup — there
is no in-app settings screen (deliberately: these are operational/
security decisions, not day-to-day preferences).

| Variable | Purpose | Default |
|---|---|---|
| `PUMC_HTTP_ADDR` | Address the server binds to, e.g. `0.0.0.0:8443` | *(required, no default)* |
| `PUMC_ENVIRONMENT` | `development` or `production` — affects logging verbosity only | `development` |
| `PUMC_DB_DRIVER` | `sqlite` or `postgres` | `sqlite` |
| `PUMC_DB_DSN` | File path (sqlite) or connection string (postgres) | `./profileunity-msp-console.db` |
| `PUMC_LOG_LEVEL` | `debug`, `info`, `warn`, or `error` | `info` |
| `PUMC_COLLECTION_INTERVAL` | How often the scheduler checks whether it's time to collect (Go duration syntax, e.g. `1h`) | `1h` |
| `PUMC_COLLECTION_TIMEZONE` | IANA timezone used to compute the collection-day boundary. Stored timestamps are always UTC regardless. | `UTC` |
| `PUMC_COLLECTION_CONCURRENCY` | Max tenants polled at once | `5` |
| `PUMC_COLLECTION_TENANT_TIMEOUT` | Per-tenant timeout including retries — one dead tenant can never stall the whole run | `30s` |
| `PUMC_CREDENTIAL_ENCRYPTION_KEY` | Base64 32-byte (AES-256) key encrypting stored tenant credentials at rest. Generate with `openssl rand -base64 32`. Leave unset if no tenant will ever have credentials. | *(unset)* |
| `PUMC_SESSION_IDLE_TIMEOUT` | Operator session idle timeout | `30m` |
| `PUMC_SESSION_ABSOLUTE_TIMEOUT` | Hard cap on a session regardless of activity | `12h` |
| `PUMC_BOOTSTRAP_ADMIN_USERNAME` / `PASSWORD` | Creates the first operator account at startup, only while the users table is empty. Leave both unset once a real account exists. | *(unset)* |
| `PUMC_TLS_CERT_FILE` / `PUMC_TLS_KEY_FILE` | TLS certificate/key paths. If both files already exist they're used as-is (bring your own CA-signed pair); otherwise a self-signed pair is generated there on first startup. | `./tls-cert.pem` / `./tls-key.pem` |

Losing `PUMC_CREDENTIAL_ENCRYPTION_KEY` means losing the ability to
decrypt any tenant credential already stored — keep it somewhere durable
and separate from the database itself.

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
- **License Mode**, **Console Version**, **Last Successful Collection**:
  reported as-is from the tenant's most recent successful poll.

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
average used-license count, entitled-license count as of month end, and
a list of any entitlement changes detected during that month. The
portfolio view additionally breaks down coverage and figures per tenant
in a table, so you can see at a glance which customers had a clean month
and which didn't.

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
replace the self-signed pair at `PUMC_TLS_CERT_FILE`/`PUMC_TLS_KEY_FILE`
with a CA-signed one. Safe to accept for internal/trusted-network use in
the meantime.

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

**Nobody can sign in.** The server logs a warning at startup if the
users table is empty and no `PUMC_BOOTSTRAP_ADMIN_USERNAME`/`PASSWORD`
were set — in that state, nobody can sign in until an administrator
creates the first account by setting those and restarting, or via
direct database access. This bootstrap only ever fires when the users
table is empty; it will never overwrite or reset an existing account.

**I forgot my password.** There is no self-service or administrator
password reset. If you're still signed in somewhere, use Change
Password (click your username in the header) instead of getting locked
out. If you're already locked out and this is a test/lab instance with
nothing worth keeping, the only way back in is to delete the SQLite
database file (`PUMC_DB_DSN`, default `./profileunity-msp-console.db`)
and restart — this re-triggers the bootstrap above using whatever
`PUMC_BOOTSTRAP_ADMIN_USERNAME`/`PASSWORD` are set, but it also erases
every tenant and every collected snapshot. There is currently no way to
reset just the password without losing that data.
