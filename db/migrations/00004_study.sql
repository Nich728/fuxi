-- Decks, notes, cards, the review log and derived card state (SYSTEM_DESIGN §4.3).
-- Owned by internal/study (review log, card state) and internal/scheduling (fsrs_params).
--
-- field_hlc / deleted_hlc are written by the server until offline sync (M5)
-- lets clients write them too.

-- +goose Up
CREATE TABLE decks (
    id           uuid PRIMARY KEY,                 -- client-generated UUIDv7
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         text NOT NULL,
    settings     jsonb NOT NULL,                   -- new_per_day, card_types[], desired_retention
    field_hlc    jsonb NOT NULL DEFAULT '{}',
    deleted_hlc  text,
    created_at   timestamptz NOT NULL
);
CREATE INDEX decks_user_idx ON decks (user_id) WHERE deleted_hlc IS NULL;

CREATE TABLE notes (
    id           uuid PRIMARY KEY,                 -- client-generated UUIDv7
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    deck_id      uuid NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    entry_id     bigint REFERENCES dict_entries(id),   -- null for custom notes
    kind         text NOT NULL CHECK (kind IN ('word', 'sentence')),
    fields       jsonb NOT NULL,                   -- {hanzi, pinyin, meaning, sentence, cloze, notes}
    tags         text[] NOT NULL DEFAULT '{}',
    origin       text NOT NULL CHECK (origin IN ('search', 'csv', 'hsk', 'tutor', 'extract', 'placement', 'manual')),
    field_hlc    jsonb NOT NULL DEFAULT '{}',
    deleted_hlc  text,
    created_at   timestamptz NOT NULL
);
CREATE INDEX notes_deck_idx ON notes (deck_id, id) WHERE deleted_hlc IS NULL;
CREATE INDEX notes_user_entry_idx ON notes (user_id, entry_id) WHERE deleted_hlc IS NULL;

CREATE TABLE cards (
    id           uuid PRIMARY KEY,                 -- UUIDv5(note_id, card_type): the same on every device
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    note_id      uuid NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    card_type    text NOT NULL CHECK (card_type IN ('recognition', 'recall', 'tones', 'cloze')),
    suspended    boolean NOT NULL DEFAULT false,
    field_hlc    jsonb NOT NULL DEFAULT '{}',
    deleted_hlc  text,
    created_at   timestamptz NOT NULL,
    UNIQUE (note_id, card_type)
);
CREATE INDEX cards_user_idx ON cards (user_id);

-- Append-only source of truth. Never UPDATE or DELETE (ADR-0003).
CREATE TABLE review_logs (
    id           uuid PRIMARY KEY,               -- client-generated: idempotency key
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_id      uuid NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    rating       smallint NOT NULL CHECK (rating BETWEEN 1 AND 4),
    reviewed_at  timestamptz NOT NULL,            -- device time of the review
    duration_ms  integer CHECK (duration_ms >= 0),
    device_id    uuid NOT NULL,
    received_at  timestamptz NOT NULL
);
CREATE INDEX review_logs_card_idx ON review_logs (card_id, reviewed_at, id);
CREATE INDEX review_logs_user_idx ON review_logs (user_id, reviewed_at);

-- Derived cache: a pure function of (review_logs for the card, fsrs params version).
-- A row exists for every live card (state 0 until its first review); deleting a
-- card or note removes its row, since the log can always rebuild it.
CREATE TABLE card_states (
    card_id          uuid PRIMARY KEY REFERENCES cards(id) ON DELETE CASCADE,
    user_id          uuid NOT NULL,
    deck_id          uuid NOT NULL,                   -- denormalised from notes for the queue
    state            smallint NOT NULL,               -- 0 new | 1 learning | 2 review | 3 relearning
    due_at           timestamptz NOT NULL,
    stability        double precision NOT NULL,
    difficulty       double precision NOT NULL,
    reps             integer NOT NULL,
    lapses           integer NOT NULL,
    remaining_steps  integer NOT NULL DEFAULT 0,      -- learning steps left (go-fsrs v4 short-term)
    last_review_at   timestamptz,
    first_review_at  timestamptz,                     -- when the card stopped being new
    params_version   integer NOT NULL,
    suspended        boolean NOT NULL DEFAULT false   -- denormalised from cards for the queue index
);
CREATE INDEX card_states_due_idx ON card_states (user_id, due_at) WHERE NOT suspended AND state <> 0;
CREATE INDEX card_states_new_idx ON card_states (user_id, deck_id) WHERE NOT suspended AND state = 0;
CREATE INDEX card_states_introduced_idx ON card_states (user_id, deck_id, first_review_at);

CREATE TABLE fsrs_params (
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version             integer NOT NULL,
    weights             double precision[] NOT NULL,
    trained_on_reviews  integer NOT NULL,
    created_at          timestamptz NOT NULL,
    PRIMARY KEY (user_id, version)
);

-- +goose Down
DROP TABLE fsrs_params;
DROP TABLE card_states;
DROP TABLE review_logs;
DROP TABLE cards;
DROP TABLE notes;
DROP TABLE decks;
