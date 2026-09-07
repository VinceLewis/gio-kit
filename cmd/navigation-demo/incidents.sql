PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;

CREATE TABLE IF NOT EXISTS incident (
    sys_id TEXT PRIMARY KEY,
    number TEXT NOT NULL UNIQUE,
    short_description TEXT NOT NULL,
    priority INTEGER NOT NULL CHECK (priority BETWEEN 1 AND 4),
    state TEXT NOT NULL,
    opened_at TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'software',
    configuration_item TEXT NOT NULL DEFAULT '',
    details TEXT NOT NULL DEFAULT '',
    active INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS incident_priority_number
    ON incident(priority, number);
CREATE INDEX IF NOT EXISTS incident_state_number
    ON incident(state, number);

WITH RECURSIVE sequence(n) AS (
    SELECT 1
    UNION ALL
    SELECT n + 1 FROM sequence WHERE n < 10000
)
INSERT OR IGNORE INTO incident (
    sys_id, number, short_description, priority, state, opened_at,
    category, configuration_item, details, active
)
SELECT
    printf('sys_id_%05d', n),
    printf('INC%07d', n),
    CASE n % 5
        WHEN 0 THEN 'Email delivery delayed for regional office'
        WHEN 1 THEN 'Network access unavailable on floor ' || ((n % 20) + 1)
        WHEN 2 THEN 'Laptop requires security update'
        WHEN 3 THEN 'Printer queue is stalled'
        ELSE 'Account access request awaiting approval'
    END,
    (n % 4) + 1,
    CASE n % 4
        WHEN 0 THEN 'New'
        WHEN 1 THEN 'In Progress'
        WHEN 2 THEN 'On Hold'
        ELSE 'Resolved'
    END,
    datetime('2026-01-01', printf('+%d minutes', n)),
    CASE n % 3 WHEN 0 THEN 'hardware' ELSE 'software' END,
    '',
    'Generated incident used by the Gio Kit phase 3 form.',
    CASE n % 4 WHEN 3 THEN 0 ELSE 1 END
FROM sequence;
