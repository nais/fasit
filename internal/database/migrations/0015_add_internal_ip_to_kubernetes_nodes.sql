-- +goose Up
ALTER TABLE kubernetes_node_statuses
ADD COLUMN "internal_ip" TEXT NOT NULL DEFAULT ''
;
