-- +goose Up
-- +goose StatementBegin
ALTER TABLE tuser
    ADD COLUMN
        token text not null;

CREATE UNIQUE INDEX token_idx ON tuser (token);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query not implemented';
-- +goose StatementEnd
