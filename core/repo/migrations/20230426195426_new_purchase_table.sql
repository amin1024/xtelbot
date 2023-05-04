-- +goose Up
-- +goose StatementBegin
CREATE TABLE purchase
(
    id           integer  not null primary key,
    tuser_id     integer  not null REFERENCES tuser (id),

    package_id   integer  not null REFERENCES package (id),
    price        integer  not null,
    package_name text     not null,

    status       integer  not null,
    processed_at datetime,

    msg_id       integer  not null,
    created_at   datetime not null
);

CREATE TABLE keyval
(
    id    integer not null primary key,
    key   text    not null,
    value text    not null,

    UNIQUE (key, value)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE purchase;
DROP TABLE keyval;
-- +goose StatementEnd
