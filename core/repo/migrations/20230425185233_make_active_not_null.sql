-- +goose Up
-- +goose StatementBegin

-- create a new table so that we modify the active column (sqlite thing!)
CREATE TABLE IF NOT EXISTS new_package
(
    id              integer not null primary key,
    name            text    not null,
    duration        integer not null,
    price           integer not null,
    traffic_allowed float   not null default 1,
    reset_mode      text    not null,
    active          boolean not null default true
);

INSERT INTO new_package(id, name, duration, price, traffic_allowed, reset_mode, active)
SELECT id, name, duration, price, traffic_allowed, reset_mode, active
FROM package;

DROP TABLE package;
ALTER TABLE new_package
    RENAME TO package;

CREATE TABLE IF NOT EXISTS new_xnode
(
    id         integer not null primary key,
    address    text    not null unique,
    panel_type text    not null,
    active     boolean not null default true
);
INSERT INTO new_xnode (id, address, panel_type, active)
SELECT id, address, panel_type, active
FROM xnode;

DROP TABLE xnode;
ALTER TABLE new_xnode
    RENAME TO xnode;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
