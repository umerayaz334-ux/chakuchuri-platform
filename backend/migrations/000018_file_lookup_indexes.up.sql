CREATE INDEX IF NOT EXISTS idx_files_tenant_owner_created
  ON files (tenant_id, owner_type, created_at DESC, external_id);

CREATE INDEX IF NOT EXISTS idx_files_tenant_checksum
  ON files (tenant_id, checksum_sha256);
