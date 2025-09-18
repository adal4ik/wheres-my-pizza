-- Расширения
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Схемы
CREATE SCHEMA IF NOT EXISTS ordering;
CREATE SCHEMA IF NOT EXISTS kitchen;
CREATE SCHEMA IF NOT EXISTS tracking;

-- Типы
CREATE TYPE ordering.order_status   AS ENUM ('received','cooking','ready','completed','cancelled');
CREATE TYPE ordering.order_type     AS ENUM ('dine_in','takeout','delivery');
CREATE TYPE ordering.order_priority AS ENUM ('low','normal','high');

-- Ядро заказов
CREATE TABLE ordering.orders (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_name      text NOT NULL CHECK (char_length(customer_name) BETWEEN 1 AND 100),
  order_type         ordering.order_type NOT NULL,
  table_number       integer,
  delivery_address   text,
  total_amount       numeric(10,2) NOT NULL CHECK (total_amount >= 0),
  priority           ordering.order_priority NOT NULL DEFAULT 'normal',
  status             ordering.order_status NOT NULL DEFAULT 'received',
  assigned_worker_id uuid, -- FK добавим после kitchen.workers
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  completed_at       timestamptz,
  CHECK ((order_type <> 'dine_in')  OR table_number     IS NOT NULL),
  CHECK ((order_type <> 'delivery') OR delivery_address IS NOT NULL)
);

CREATE TABLE ordering.order_items (
  id        bigserial PRIMARY KEY,
  order_id  uuid NOT NULL REFERENCES ordering.orders(id) ON DELETE CASCADE,
  item_name text NOT NULL CHECK (char_length(item_name) BETWEEN 1 AND 100),
  quantity  integer NOT NULL CHECK (quantity BETWEEN 1 AND 100),
  price     numeric(10,2) NOT NULL CHECK (price >= 0.01)
);

CREATE TABLE ordering.order_status_log (
  id         bigserial PRIMARY KEY,
  order_id   uuid NOT NULL REFERENCES ordering.orders(id) ON DELETE CASCADE,
  status     ordering.order_status NOT NULL,
  note       text,
  changed_by text,
  changed_at timestamptz NOT NULL DEFAULT now()
);

-- Воркеры кухни (минимум)
CREATE TYPE kitchen.worker_status AS ENUM ('online','offline');
CREATE TABLE kitchen.workers (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name            text NOT NULL UNIQUE,
  specialization  text[] NOT NULL DEFAULT '{}',
  status          kitchen.worker_status NOT NULL DEFAULT 'offline',
  last_heartbeat  timestamptz NOT NULL DEFAULT now(),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);

-- Привязка заказа к воркеру (может быть NULL)
ALTER TABLE ordering.orders
  ADD CONSTRAINT orders_worker_fk
  FOREIGN KEY (assigned_worker_id) REFERENCES kitchen.workers(id) ON DELETE SET NULL;

-- Индексы (базово)
CREATE INDEX ordering_orders_status_idx   ON ordering.orders (status);
CREATE INDEX ordering_orders_created_idx  ON ordering.orders (created_at);
CREATE INDEX ordering_items_order_idx     ON ordering.order_items (order_id);
CREATE INDEX ordering_status_log_idx      ON ordering.order_status_log (order_id, changed_at DESC);
