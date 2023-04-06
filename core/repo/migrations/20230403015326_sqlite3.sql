-- +goose Up
-- +goose StatementBegin
PRAGMA
foreign_keys = ON;

CREATE TABLE tuser
(
    id                   integer not null primary key,
    tid                  unsigned big int not null UNIQUE,
    username             text    not null UNIQUE,
    uuid                 text    not null,
    active               boolean not null default false,
    added_to_nodes_count int     not null default 0,

    traffic_usage        float   not null default 0,
    expire_at            TIME    not null,
    package_id           integer not null,

    FOREIGN KEY (package_id) REFERENCES package (id)
);

CREATE TABLE xnode
(
    id         integer not null primary key,
    address    text    not null unique,
    panel_type text    not null,
    active     boolean default true
);

CREATE TABLE package
(
    id              integer not null primary key,
    name            text    not null,
    duration        integer not null,
    price           integer not null,
    traffic_allowed float   not null default 1,
    reset_mode      text    not null,
    active          boolean          default true
);

INSERT INTO package(name, duration, price, traffic_allowed, reset_mode, active)
values ('basic_test', 15, 0, 5, 'no_reset', true);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query NOT IMPLEMENTED';
-- +goose StatementEnd
