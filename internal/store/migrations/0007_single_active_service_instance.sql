-- +goose Up
UPDATE service_instances
SET status = 'planned',
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE status = 'active'
  AND (
    listen_port <= 0
    OR id NOT IN (
      SELECT id
      FROM (
        SELECT id
        FROM service_instances
        WHERE status = 'active'
          AND listen_port > 0
        ORDER BY updated_at DESC, created_at DESC, id DESC
        LIMIT 1
      )
    )
  );

-- +goose Down
-- Historical active rows cannot be reconstructed safely.
