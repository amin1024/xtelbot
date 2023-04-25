-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS renovate_rule
(
    id        integer primary key,
    remark    text not null,
    old_value text not null,
    new_value text not null,
    ignore boolean default false not null,

    UNIQUE (remark, old_value)
);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query NOT IMPLEMENTED';
-- +goose StatementEnd
