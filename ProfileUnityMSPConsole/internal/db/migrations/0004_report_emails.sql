-- Tracks which monthly portfolio reports have already been emailed, so a
-- scheduler tick that fires more than once for the same month (a
-- restart, a duplicate tick) never sends the report twice. Mirrors the
-- snapshots table's UNIQUE(tenant_id, collection_date) idempotency
-- pattern from 0002, keyed here by (year, month) instead. Only successful
-- sends get a row -- a failed attempt leaves the month unrecorded so the
-- next tick retries it, rather than a status column silently permanently
-- suppressing a report nobody actually received.
CREATE TABLE report_emails (
    id            TEXT PRIMARY KEY,
    year          INTEGER NOT NULL,
    month         INTEGER NOT NULL,
    sent_at_utc   TEXT NOT NULL,
    recipients    TEXT NOT NULL,

    UNIQUE (year, month)
);
