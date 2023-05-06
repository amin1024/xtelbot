-- +goose Up
-- +goose StatementBegin
ALTER TABLE purchase ADD COLUMN transaction_id text;
ALTER TABLE purchase ADD COLUMN shaparak_ref text;

create unique index trans_id_unique_idx ON purchase (transaction_id);
alter table purchase drop column msg_id;
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
