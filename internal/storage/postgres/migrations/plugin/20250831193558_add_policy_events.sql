-- +goose Up
-- +goose StatementBegin
ALTER TYPE system_event_type ADD VALUE 'policy_created';
ALTER TYPE system_event_type ADD VALUE 'policy_deleted';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
