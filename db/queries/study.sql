-- Queries for internal/study (SYSTEM_DESIGN §4.3, §6.2, §6.3).
-- Every query takes the user id: there are no unscoped reads or writes
-- (SYSTEM_DESIGN §12.1). Lookups by id alone exist only to tell an
-- idempotent retry from an id clash, and return no user data.

-- Decks -----------------------------------------------------------------

-- name: InsertDeck :one
INSERT INTO decks (id, user_id, name, settings, field_hlc, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO NOTHING
RETURNING *;

-- name: DeckOwner :one
SELECT user_id FROM decks WHERE id = $1;

-- name: GetDeck :one
SELECT * FROM decks WHERE id = $1 AND user_id = $2 AND deleted_hlc IS NULL;

-- name: CountDeckNotes :one
SELECT count(*) FROM notes
WHERE deck_id = $1 AND user_id = $2 AND deleted_hlc IS NULL;

-- name: ListDecks :many
SELECT d.*,
       (SELECT count(*) FROM notes n
        WHERE n.deck_id = d.id AND n.deleted_hlc IS NULL)::bigint AS note_count
FROM decks d
WHERE d.user_id = $1 AND d.deleted_hlc IS NULL
ORDER BY d.created_at, d.id;

-- name: UpdateDeck :one
UPDATE decks
SET name      = coalesce(sqlc.narg(name), name),
    settings  = coalesce(sqlc.narg(settings), settings),
    field_hlc = field_hlc || sqlc.arg(field_hlc)::jsonb
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND deleted_hlc IS NULL
RETURNING *;

-- name: DeleteDeck :execrows
UPDATE decks SET deleted_hlc = sqlc.arg(hlc)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND deleted_hlc IS NULL;

-- name: DeleteDeckNotes :exec
UPDATE notes SET deleted_hlc = sqlc.arg(hlc)
WHERE deck_id = sqlc.arg(deck_id) AND user_id = sqlc.arg(user_id) AND deleted_hlc IS NULL;

-- name: DeleteDeckCards :exec
UPDATE cards c SET deleted_hlc = sqlc.arg(hlc)
WHERE c.user_id = sqlc.arg(user_id)::uuid AND c.deleted_hlc IS NULL
  AND c.note_id IN (SELECT n.id FROM notes n WHERE n.deck_id = sqlc.arg(deck_id)::uuid AND n.user_id = sqlc.arg(user_id)::uuid);

-- name: DeleteDeckCardStates :exec
DELETE FROM card_states WHERE deck_id = $1 AND user_id = $2;

-- Notes -----------------------------------------------------------------

-- name: InsertNote :one
INSERT INTO notes (id, user_id, deck_id, entry_id, kind, fields, tags, origin, field_hlc, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (id) DO NOTHING
RETURNING *;

-- name: NoteOwner :one
SELECT user_id FROM notes WHERE id = $1;

-- name: ListNotesByEntry :many
-- Where a dictionary word already is, across the user's decks.
SELECT * FROM notes
WHERE user_id = $1 AND entry_id = $2 AND deleted_hlc IS NULL
ORDER BY id;

-- name: GetNote :one
SELECT * FROM notes WHERE id = $1 AND user_id = $2 AND deleted_hlc IS NULL;

-- name: ListNotes :many
-- Cursor pagination on the UUIDv7 id, which sorts by creation time.
SELECT * FROM notes
WHERE deck_id = sqlc.arg(deck_id) AND user_id = sqlc.arg(user_id) AND deleted_hlc IS NULL
  AND id > sqlc.arg(after)
ORDER BY id
LIMIT sqlc.arg(lim);

-- name: DeckHasHanzi :one
SELECT EXISTS (
    SELECT 1 FROM notes
    WHERE deck_id = $1 AND user_id = $2 AND deleted_hlc IS NULL
      AND fields->>'hanzi' = sqlc.arg(hanzi)::text);

-- name: UpdateNote :one
UPDATE notes
SET fields    = fields || sqlc.arg(fields)::jsonb,
    tags      = coalesce(sqlc.narg(tags)::text[], tags),
    field_hlc = field_hlc || sqlc.arg(field_hlc)::jsonb
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND deleted_hlc IS NULL
RETURNING *;

-- name: DeleteNote :execrows
UPDATE notes SET deleted_hlc = sqlc.arg(hlc)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND deleted_hlc IS NULL;

-- name: DeleteNoteCards :exec
UPDATE cards SET deleted_hlc = sqlc.arg(hlc)
WHERE note_id = sqlc.arg(note_id) AND user_id = sqlc.arg(user_id) AND deleted_hlc IS NULL;

-- name: DeleteNoteCardStates :exec
DELETE FROM card_states cs
WHERE cs.user_id = sqlc.arg(user_id)::uuid
  AND cs.card_id IN (SELECT c.id FROM cards c WHERE c.note_id = sqlc.arg(note_id)::uuid AND c.user_id = sqlc.arg(user_id)::uuid);

-- Cards -----------------------------------------------------------------

-- name: InsertCard :exec
INSERT INTO cards (id, user_id, note_id, card_type, field_hlc, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO NOTHING;

-- name: InsertCardState :exec
-- A new card: state 0, due when created, FSRS memory not yet started.
INSERT INTO card_states (card_id, user_id, deck_id, state, due_at, stability, difficulty,
                         reps, lapses, params_version)
VALUES ($1, $2, $3, 0, $4, 0, 0, 0, 0, $5)
ON CONFLICT (card_id) DO NOTHING;

-- name: ListNoteCards :many
SELECT c.id, c.card_type, c.suspended, cs.state, cs.due_at, cs.reps, cs.lapses, cs.last_review_at
FROM cards c
JOIN card_states cs ON cs.card_id = c.id
WHERE c.note_id = $1 AND c.user_id = $2 AND c.deleted_hlc IS NULL
ORDER BY CASE c.card_type WHEN 'recognition' THEN 0 WHEN 'recall' THEN 1 WHEN 'tones' THEN 2 ELSE 3 END;

-- name: GetCardForReview :one
-- The card with what a review needs: its deck's settings and creation time.
SELECT c.id, c.created_at, n.deck_id, d.settings AS deck_settings
FROM cards c
JOIN notes n ON n.id = c.note_id
JOIN decks d ON d.id = n.deck_id
WHERE c.id = $1 AND c.user_id = $2 AND c.deleted_hlc IS NULL;

-- name: SetCardSuspended :execrows
UPDATE cards SET suspended = sqlc.arg(suspended), field_hlc = field_hlc || sqlc.arg(field_hlc)::jsonb
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND deleted_hlc IS NULL;

-- name: SetNewCardsSuspendedForEntry :execrows
-- "I already know this" (SYSTEM_DESIGN §9.7): suspend, or bring back, the
-- never-reviewed cards of every note for a dictionary entry.
WITH changed AS (
    UPDATE cards c
    SET suspended = sqlc.arg(suspended), field_hlc = c.field_hlc || sqlc.arg(field_hlc)::jsonb
    FROM notes n, card_states cs
    WHERE c.user_id = sqlc.arg(user_id) AND n.id = c.note_id AND n.user_id = sqlc.arg(user_id)
      AND n.entry_id = sqlc.arg(entry_id) AND n.deleted_hlc IS NULL AND c.deleted_hlc IS NULL
      AND cs.card_id = c.id AND cs.state = 0 AND c.suspended <> sqlc.arg(suspended)
    RETURNING c.id
)
UPDATE card_states st SET suspended = sqlc.arg(suspended)
FROM changed
WHERE st.user_id = sqlc.arg(user_id) AND st.card_id = changed.id;

-- name: SetCardStateSuspended :exec
UPDATE card_states SET suspended = $3 WHERE card_id = $1 AND user_id = $2;

-- Reviews (append-only, ADR-0003) ---------------------------------------

-- name: InsertReviewLog :one
INSERT INTO review_logs (id, user_id, card_id, rating, reviewed_at, duration_ms, device_id, received_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO NOTHING
RETURNING id;

-- name: GetReviewLogByID :one
-- Only used after an insert conflict, to tell a retry from an id clash.
SELECT user_id, card_id FROM review_logs WHERE id = $1;

-- name: ListCardReviews :many
SELECT id, rating, reviewed_at FROM review_logs
WHERE card_id = $1 AND user_id = $2
ORDER BY reviewed_at, id;

-- name: LockCardState :one
SELECT * FROM card_states WHERE card_id = $1 AND user_id = $2 FOR UPDATE;

-- name: GetCardState :one
SELECT * FROM card_states WHERE card_id = $1 AND user_id = $2;

-- name: UpdateCardState :one
UPDATE card_states
SET state = sqlc.arg(state), due_at = sqlc.arg(due_at), stability = sqlc.arg(stability),
    difficulty = sqlc.arg(difficulty), reps = sqlc.arg(reps), lapses = sqlc.arg(lapses),
    remaining_steps = sqlc.arg(remaining_steps), last_review_at = sqlc.narg(last_review_at),
    first_review_at = sqlc.narg(first_review_at), params_version = sqlc.arg(params_version)
WHERE card_id = sqlc.arg(card_id) AND user_id = sqlc.arg(user_id)
RETURNING *;

-- Queue (SYSTEM_DESIGN §6.3) --------------------------------------------

-- name: CountDue :one
SELECT count(*) FROM card_states
WHERE user_id = sqlc.arg(user_id) AND NOT suspended AND state <> 0
  AND due_at < sqlc.arg(day_end)
  AND (sqlc.narg(deck_id)::uuid IS NULL OR deck_id = sqlc.narg(deck_id));

-- name: ListDue :many
SELECT cs.card_id, cs.deck_id, cs.state, cs.due_at, cs.reps, cs.lapses,
       cs.stability, cs.difficulty, cs.remaining_steps, cs.last_review_at, cs.first_review_at,
       c.card_type, n.id AS note_id, n.kind, n.entry_id, n.fields
FROM card_states cs
JOIN cards c ON c.id = cs.card_id
JOIN notes n ON n.id = c.note_id
WHERE cs.user_id = sqlc.arg(user_id) AND NOT cs.suspended AND cs.state <> 0
  AND cs.due_at < sqlc.arg(day_end)
  AND (sqlc.narg(deck_id)::uuid IS NULL OR cs.deck_id = sqlc.narg(deck_id))
ORDER BY cs.due_at, cs.card_id
LIMIT sqlc.arg(lim);

-- name: ListQueueDecks :many
SELECT id, settings FROM decks
WHERE user_id = sqlc.arg(user_id) AND deleted_hlc IS NULL
  AND (sqlc.narg(deck_id)::uuid IS NULL OR id = sqlc.narg(deck_id))
ORDER BY created_at, id;

-- name: CountDueByDeck :many
SELECT deck_id, count(*) AS n FROM card_states
WHERE user_id = sqlc.arg(user_id) AND NOT suspended AND state <> 0
  AND due_at < sqlc.arg(day_end)
GROUP BY deck_id;

-- name: ReviewsPerDay :many
-- Reviews per study day (SYSTEM_DESIGN §6.4, §6.6): the local date of the
-- review time moved back by the day-start hour, so 1 am counts for the evening before.
SELECT ((reviewed_at AT TIME ZONE sqlc.arg(tz)::text) - make_interval(hours => sqlc.arg(day_starts_at)::int))::date AS day,
       count(*) AS n
FROM review_logs
WHERE user_id = sqlc.arg(user_id) AND reviewed_at >= sqlc.arg(since)::timestamptz
GROUP BY 1
ORDER BY 1;

-- name: CountIntroduced :many
-- New cards started today, per deck (first_review_at is set by the first review).
SELECT deck_id, count(*) AS n FROM card_states
WHERE user_id = sqlc.arg(user_id) AND first_review_at >= sqlc.arg(day_start)::timestamptz
GROUP BY deck_id;

-- name: CountNew :many
SELECT deck_id, count(*) AS n FROM card_states
WHERE user_id = $1 AND NOT suspended AND state = 0
GROUP BY deck_id;

-- name: ListNew :many
-- Oldest notes first; a note's recognition card before its recall card.
SELECT cs.card_id, cs.deck_id, cs.state, cs.due_at, cs.reps, cs.lapses,
       cs.stability, cs.difficulty, cs.remaining_steps, cs.last_review_at, cs.first_review_at,
       c.card_type, n.id AS note_id, n.kind, n.entry_id, n.fields
FROM card_states cs
JOIN cards c ON c.id = cs.card_id
JOIN notes n ON n.id = c.note_id
WHERE cs.user_id = sqlc.arg(user_id) AND cs.deck_id = sqlc.arg(deck_id)
  AND NOT cs.suspended AND cs.state = 0
ORDER BY n.id,
         CASE c.card_type WHEN 'recognition' THEN 0 WHEN 'recall' THEN 1 WHEN 'tones' THEN 2 ELSE 3 END
LIMIT sqlc.arg(lim);
