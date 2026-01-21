-- +goose Up
-- +goose StatementBegin
CREATE TYPE system_event_type AS ENUM ('vault_reshared');

CREATE TABLE IF NOT EXISTS system_events (
    id BIGSERIAL PRIMARY KEY,
    public_key TEXT,
    policy_id UUID,
    event_type system_event_type NOT NULL,
    event_data JSONB NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_system_events_public_key ON system_events (public_key);
CREATE INDEX IF NOT EXISTS idx_system_events_policy_id ON system_events (policy_id);
CREATE INDEX IF NOT EXISTS idx_system_events_event_type ON system_events (event_type);
CREATE INDEX IF NOT EXISTS idx_system_events_created_at ON system_events (created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS system_events;
-- +goose StatementEnd
