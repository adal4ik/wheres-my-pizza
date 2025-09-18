DROP INDEX IF EXISTS ordering_status_log_idx;
DROP INDEX IF EXISTS ordering_items_order_idx;
DROP INDEX IF EXISTS ordering_orders_created_idx;
DROP INDEX IF EXISTS ordering_orders_status_idx;

ALTER TABLE IF EXISTS ordering.orders DROP CONSTRAINT IF EXISTS orders_worker_fk;

DROP TABLE IF EXISTS kitchen.workers;
DROP TYPE  IF EXISTS kitchen.worker_status;

DROP TABLE IF EXISTS ordering.order_status_log;
DROP TABLE IF EXISTS ordering.order_items;
DROP TABLE IF EXISTS ordering.orders;
DROP TYPE  IF EXISTS ordering.order_priority;
DROP TYPE  IF EXISTS ordering.order_type;
DROP TYPE  IF EXISTS ordering.order_status;

DROP SCHEMA IF EXISTS tracking;
DROP SCHEMA IF EXISTS kitchen;
DROP SCHEMA IF EXISTS ordering;

DROP EXTENSION IF EXISTS "pgcrypto";
