-- +goose Up
ALTER TYPE environment_kind
ADD VALUE 'onprem'
;
