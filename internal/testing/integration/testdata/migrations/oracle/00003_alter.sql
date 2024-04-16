-- +goose Up
ALTER TABLE repos
    ADD (
        homepage_url NVARCHAR2(255),
        is_private NUMBER(1) DEFAULT 0 NOT NULL
        );

-- +goose Down
ALTER TABLE repos
    DROP COLUMN homepage_url;
ALTER TABLE repos
    DROP COLUMN is_private;
