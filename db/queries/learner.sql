-- Queries owned by internal/learner (SYSTEM_DESIGN §4.6).

-- name: UpsertKnowledge :one
INSERT INTO word_knowledge (id, user_id, entry_id, status, source, confidence, field_hlc, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(entry_id), sqlc.arg(status), sqlc.arg(source),
        sqlc.arg(confidence), sqlc.arg(field_hlc), sqlc.arg(updated_at)::timestamptz)
ON CONFLICT (user_id, entry_id) DO UPDATE
SET status = EXCLUDED.status, source = EXCLUDED.source, confidence = EXCLUDED.confidence,
    field_hlc = word_knowledge.field_hlc || EXCLUDED.field_hlc, updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: DeleteKnowledge :execrows
DELETE FROM word_knowledge WHERE user_id = $1 AND entry_id = $2;

-- name: ListKnowledge :many
SELECT * FROM word_knowledge
WHERE user_id = sqlc.arg(user_id) AND entry_id = ANY(sqlc.arg(entry_ids)::bigint[])
ORDER BY entry_id;
