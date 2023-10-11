-- +goose Up
-- +goose StatementBegin
ALTER TABLE blocks ADD COLUMN IF NOT EXISTS finalized bool not null default false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE blocks DROP COLUMN finalized;
-- +goose StatementEnd
