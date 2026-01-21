-- +goose Up
-- +goose StatementBegin
ALTER TYPE system_event_type ADD VALUE 'vault_deleted';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
