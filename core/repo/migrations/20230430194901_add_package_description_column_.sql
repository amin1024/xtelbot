-- +goose Up
-- +goose StatementBegin
ALTER TABLE package ADD COLUMN description text not null default '??';

UPDATE package SET name = '_free_', price = 0, traffic_allowed = 1, duration = 1 WHERE name = 'basic_test';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
