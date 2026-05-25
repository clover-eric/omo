-- +goose Up
UPDATE service_instances
SET status = 'planned',
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE status = 'active'
  AND id NOT IN (
    SELECT id
    FROM (
      SELECT id
      FROM service_instances
      WHERE status = 'active'
        AND profile_id = 'standard-secure-access'
        AND listen_port > 0
        AND TRIM(access_username) <> ''
        AND TRIM(access_password) <> ''
        AND TRIM(access_path) <> ''
        AND length(config_version) = 14
        AND config_version GLOB '[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]'
      ORDER BY updated_at DESC, created_at DESC, id DESC
      LIMIT 1
    )
  );

-- +goose Down
-- Historical active rows cannot be reconstructed safely.
