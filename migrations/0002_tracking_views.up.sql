-- Упрощённое представление для текущего статуса
CREATE OR REPLACE VIEW tracking.v_order_current_status AS
WITH last_status AS (
  SELECT
    l.order_id,
    (ARRAY_AGG(l.status ORDER BY l.changed_at DESC))[1] AS status,
    MAX(l.changed_at) AS last_changed_at
  FROM ordering.order_status_log l
  GROUP BY l.order_id
)
SELECT
  o.id            AS order_id,
  o.customer_name,
  o.order_type,
  o.priority,
  o.total_amount,
  o.created_at,
  o.updated_at,
  ls.status       AS current_status,
  ls.last_changed_at
FROM ordering.orders o
LEFT JOIN last_status ls ON ls.order_id = o.id;
