-- Clean cutover from Phase 2 todo entity
DROP TABLE IF EXISTS todos;

-- Per-user enum guard (used by card_labels)
DO $$ BEGIN
    CREATE TYPE label_color AS ENUM ('red','orange','yellow','green','blue','purple');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

-- Columns: ordered per user
CREATE TABLE columns (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL CHECK (char_length(trim(name)) BETWEEN 1 AND 60),
    position   INT         NOT NULL CHECK (position >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, position)
);
CREATE INDEX idx_columns_user_id ON columns(user_id);

-- Cards: belong to a column (which carries user ownership transitively)
CREATE TABLE cards (
    id          BIGSERIAL    PRIMARY KEY,
    column_id   BIGINT       NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
    title       TEXT         NOT NULL CHECK (char_length(trim(title)) BETWEEN 1 AND 200),
    description TEXT         NOT NULL DEFAULT '' CHECK (char_length(description) <= 10000),
    due_at      TIMESTAMPTZ  NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_cards_column_id ON cards(column_id);

-- Labels on a card: zero..N from the fixed enum
CREATE TABLE card_labels (
    card_id BIGINT      NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    color   label_color NOT NULL,
    PRIMARY KEY (card_id, color)
);
