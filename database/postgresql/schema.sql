-- FLVX public-safe PostgreSQL schema.
-- Generated from the public-safe GORM models. No production data is included.
-- The backend still runs AutoMigrate on startup and remains the runtime source
-- of truth for schema reconciliation.

BEGIN;

CREATE TABLE IF NOT EXISTS "user" (
  "id" BIGSERIAL PRIMARY KEY,
  "user" VARCHAR(100) NOT NULL,
  "pwd" VARCHAR(100) NOT NULL,
  "role_id" INTEGER NOT NULL,
  "exp_time" BIGINT NOT NULL,
  "flow" BIGINT NOT NULL,
  "in_flow" BIGINT NOT NULL DEFAULT 0,
  "out_flow" BIGINT NOT NULL DEFAULT 0,
  "flow_reset_time" BIGINT NOT NULL,
  "num" INTEGER NOT NULL,
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT,
  "status" INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS "user_quota" (
  "user_id" BIGINT PRIMARY KEY,
  "daily_limit_gb" BIGINT NOT NULL DEFAULT 0,
  "monthly_limit_gb" BIGINT NOT NULL DEFAULT 0,
  "daily_used_bytes" BIGINT NOT NULL DEFAULT 0,
  "monthly_used_bytes" BIGINT NOT NULL DEFAULT 0,
  "day_key" BIGINT NOT NULL DEFAULT 0,
  "month_key" BIGINT NOT NULL DEFAULT 0,
  "disabled_by_quota" INTEGER NOT NULL DEFAULT 0,
  "disabled_at" BIGINT NOT NULL DEFAULT 0,
  "paused_forward_ids" TEXT NOT NULL DEFAULT '',
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "forward" (
  "id" BIGSERIAL PRIMARY KEY,
  "user_id" BIGINT NOT NULL,
  "user_name" VARCHAR(100) NOT NULL,
  "name" VARCHAR(100) NOT NULL,
  "tunnel_id" BIGINT NOT NULL,
  "mode" VARCHAR(20) NOT NULL DEFAULT 'direct',
  "remote_addr" TEXT NOT NULL,
  "sni_rules" TEXT NOT NULL DEFAULT '',
  "strategy" VARCHAR(100) NOT NULL DEFAULT 'fifo',
  "in_flow" BIGINT NOT NULL DEFAULT 0,
  "out_flow" BIGINT NOT NULL DEFAULT 0,
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL,
  "status" INTEGER NOT NULL,
  "inx" INTEGER NOT NULL DEFAULT 0,
  "speed_id" BIGINT
);

CREATE TABLE IF NOT EXISTS "forward_port" (
  "id" BIGSERIAL PRIMARY KEY,
  "forward_id" BIGINT NOT NULL,
  "node_id" BIGINT NOT NULL,
  "port" INTEGER NOT NULL,
  "in_ip" TEXT
);

CREATE TABLE IF NOT EXISTS "node" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" VARCHAR(100) NOT NULL,
  "remark" TEXT,
  "expiry_time" BIGINT,
  "renewal_cycle" VARCHAR(20),
  "secret" VARCHAR(100) NOT NULL,
  "server_ip" VARCHAR(100) NOT NULL,
  "server_ip_v4" VARCHAR(100),
  "server_ip_v6" VARCHAR(100),
  "extra_ips" TEXT,
  "port" TEXT NOT NULL,
  "interface_name" VARCHAR(200),
  "version" VARCHAR(100),
  "http" INTEGER NOT NULL DEFAULT 0,
  "tls" INTEGER NOT NULL DEFAULT 0,
  "socks" INTEGER NOT NULL DEFAULT 0,
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT,
  "status" INTEGER NOT NULL,
  "tcp_listen_addr" VARCHAR(100) NOT NULL DEFAULT '[::]',
  "udp_listen_addr" VARCHAR(100) NOT NULL DEFAULT '[::]',
  "inx" INTEGER NOT NULL DEFAULT 0,
  "is_remote" INTEGER DEFAULT 0,
  "remote_url" TEXT,
  "remote_token" TEXT,
  "remote_config" TEXT,
  "expiry_reminder_dismissed" INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS "speed_limit" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" VARCHAR(100) NOT NULL,
  "speed" INTEGER NOT NULL,
  "tunnel_id" BIGINT,
  "tunnel_name" VARCHAR(100),
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT,
  "status" INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS "statistics_flow" (
  "id" BIGSERIAL PRIMARY KEY,
  "user_id" BIGINT NOT NULL,
  "flow" BIGINT NOT NULL,
  "total_flow" BIGINT NOT NULL,
  "time" VARCHAR(100) NOT NULL,
  "created_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "tunnel" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" VARCHAR(100) NOT NULL,
  "traffic_ratio" DOUBLE PRECISION NOT NULL DEFAULT 1.0,
  "type" INTEGER NOT NULL,
  "protocol" VARCHAR(10) NOT NULL DEFAULT 'tls',
  "flow" BIGINT NOT NULL,
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL,
  "status" INTEGER NOT NULL,
  "in_ip" TEXT,
  "inx" INTEGER NOT NULL DEFAULT 0,
  "ip_preference" VARCHAR(10) NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS "chain_tunnel" (
  "id" BIGSERIAL PRIMARY KEY,
  "tunnel_id" BIGINT NOT NULL,
  "chain_type" VARCHAR(10) NOT NULL,
  "node_id" BIGINT NOT NULL,
  "port" BIGINT,
  "strategy" VARCHAR(10),
  "inx" BIGINT,
  "protocol" VARCHAR(10),
  "connect_ip" VARCHAR(45)
);

CREATE TABLE IF NOT EXISTS "user_tunnel" (
  "id" BIGSERIAL PRIMARY KEY,
  "user_id" BIGINT NOT NULL,
  "tunnel_id" BIGINT NOT NULL,
  "speed_id" BIGINT,
  "num" INTEGER NOT NULL,
  "flow" BIGINT NOT NULL,
  "in_flow" BIGINT NOT NULL DEFAULT 0,
  "out_flow" BIGINT NOT NULL DEFAULT 0,
  "flow_reset_time" BIGINT NOT NULL,
  "exp_time" BIGINT NOT NULL,
  "status" INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS "tunnel_group" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" VARCHAR(100) NOT NULL,
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL,
  "status" INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS "user_group" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" VARCHAR(100) NOT NULL,
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL,
  "status" INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS "tunnel_group_tunnel" (
  "id" BIGSERIAL PRIMARY KEY,
  "tunnel_group_id" BIGINT NOT NULL,
  "tunnel_id" BIGINT NOT NULL,
  "created_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "user_group_user" (
  "id" BIGSERIAL PRIMARY KEY,
  "user_group_id" BIGINT NOT NULL,
  "user_id" BIGINT NOT NULL,
  "created_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "group_permission" (
  "id" BIGSERIAL PRIMARY KEY,
  "user_group_id" BIGINT NOT NULL,
  "tunnel_group_id" BIGINT NOT NULL,
  "created_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "group_permission_grant" (
  "id" BIGSERIAL PRIMARY KEY,
  "user_group_id" BIGINT NOT NULL,
  "tunnel_group_id" BIGINT NOT NULL,
  "user_tunnel_id" BIGINT NOT NULL,
  "created_by_group" INTEGER NOT NULL DEFAULT 0,
  "created_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "monitor_permission" (
  "id" BIGSERIAL PRIMARY KEY,
  "user_id" BIGINT NOT NULL,
  "created_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "vite_config" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" VARCHAR(200) NOT NULL,
  "value" TEXT NOT NULL,
  "time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "peer_share" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" TEXT NOT NULL,
  "node_id" BIGINT NOT NULL,
  "token" TEXT NOT NULL,
  "max_bandwidth" BIGINT DEFAULT 0,
  "expiry_time" BIGINT DEFAULT 0,
  "port_range_start" INTEGER DEFAULT 0,
  "port_range_end" INTEGER DEFAULT 0,
  "current_flow" BIGINT DEFAULT 0,
  "is_active" INTEGER DEFAULT 1,
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL,
  "allowed_domains" TEXT DEFAULT '',
  "allowed_ips" TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS "peer_share_runtime" (
  "id" BIGSERIAL PRIMARY KEY,
  "share_id" BIGINT NOT NULL,
  "node_id" BIGINT NOT NULL,
  "reservation_id" TEXT NOT NULL,
  "resource_key" TEXT NOT NULL,
  "binding_id" TEXT NOT NULL DEFAULT '',
  "role" TEXT NOT NULL DEFAULT '',
  "chain_name" TEXT NOT NULL DEFAULT '',
  "service_name" TEXT NOT NULL DEFAULT '',
  "protocol" TEXT NOT NULL DEFAULT 'tls',
  "strategy" TEXT NOT NULL DEFAULT 'round',
  "port" INTEGER NOT NULL DEFAULT 0,
  "target" TEXT NOT NULL DEFAULT '',
  "applied" INTEGER NOT NULL DEFAULT 0,
  "status" INTEGER NOT NULL DEFAULT 1,
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "federation_tunnel_binding" (
  "id" BIGSERIAL PRIMARY KEY,
  "tunnel_id" BIGINT NOT NULL,
  "node_id" BIGINT NOT NULL,
  "chain_type" INTEGER NOT NULL,
  "hop_inx" INTEGER NOT NULL DEFAULT 0,
  "remote_url" TEXT NOT NULL,
  "resource_key" TEXT NOT NULL,
  "remote_binding_id" TEXT NOT NULL,
  "allocated_port" INTEGER NOT NULL,
  "status" INTEGER NOT NULL DEFAULT 1,
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "tunnel_cover_profile" (
  "id" BIGSERIAL PRIMARY KEY,
  "tunnel_id" BIGINT NOT NULL,
  "enabled" INTEGER NOT NULL DEFAULT 0,
  "site_label" TEXT NOT NULL DEFAULT '',
  "domains" TEXT NOT NULL DEFAULT '',
  "cert_profile" TEXT NOT NULL DEFAULT '',
  "dns_provider" TEXT NOT NULL DEFAULT '',
  "dns_profile" TEXT NOT NULL DEFAULT '',
  "template_profile" TEXT NOT NULL DEFAULT 'static',
  "upstream_origin" TEXT NOT NULL DEFAULT '',
  "static_html" TEXT NOT NULL DEFAULT '',
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "cover_domain_profile" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" TEXT NOT NULL,
  "enabled" INTEGER NOT NULL DEFAULT 1,
  "site_label" TEXT NOT NULL DEFAULT '',
  "domains" TEXT NOT NULL DEFAULT '',
  "cert_profile" TEXT NOT NULL DEFAULT '',
  "dns_provider" TEXT NOT NULL DEFAULT '',
  "dns_profile" TEXT NOT NULL DEFAULT '',
  "template_profile" TEXT NOT NULL DEFAULT 'static',
  "upstream_origin" TEXT NOT NULL DEFAULT '',
  "static_html" TEXT NOT NULL DEFAULT '',
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "tunnel_cover_binding" (
  "id" BIGSERIAL PRIMARY KEY,
  "tunnel_id" BIGINT NOT NULL,
  "profile_id" BIGINT NOT NULL,
  "enabled" INTEGER NOT NULL DEFAULT 1,
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "node_cover_service" (
  "id" BIGSERIAL PRIMARY KEY,
  "node_id" BIGINT NOT NULL,
  "enabled" INTEGER NOT NULL DEFAULT 0,
  "public_port" INTEGER NOT NULL DEFAULT 443,
  "local_listen" TEXT NOT NULL DEFAULT '127.0.0.1:10443',
  "last_sync_time" BIGINT NOT NULL DEFAULT 0,
  "last_status" TEXT NOT NULL DEFAULT '',
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "announcement" (
  "id" BIGSERIAL PRIMARY KEY,
  "content" TEXT NOT NULL,
  "enabled" INTEGER NOT NULL DEFAULT 1,
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT
);

CREATE TABLE IF NOT EXISTS "schema_version" (
  "version" INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS "node_metric" (
  "id" BIGSERIAL PRIMARY KEY,
  "node_id" BIGINT NOT NULL,
  "timestamp" BIGINT NOT NULL,
  "cpu_usage" DOUBLE PRECISION,
  "mem_usage" DOUBLE PRECISION,
  "disk_usage" DOUBLE PRECISION,
  "net_in_bytes" BIGINT,
  "net_out_bytes" BIGINT,
  "net_in_speed" BIGINT,
  "net_out_speed" BIGINT,
  "load1" DOUBLE PRECISION,
  "load5" DOUBLE PRECISION,
  "load15" DOUBLE PRECISION,
  "tcp_conns" BIGINT,
  "udp_conns" BIGINT,
  "uptime" BIGINT
);

CREATE TABLE IF NOT EXISTS "tunnel_metric" (
  "id" BIGSERIAL PRIMARY KEY,
  "tunnel_id" BIGINT NOT NULL,
  "node_id" BIGINT NOT NULL,
  "timestamp" BIGINT NOT NULL,
  "bytes_in" BIGINT,
  "bytes_out" BIGINT,
  "connections" BIGINT,
  "errors" BIGINT,
  "avg_latency_ms" DOUBLE PRECISION
);

CREATE TABLE IF NOT EXISTS "service_monitor" (
  "id" BIGSERIAL PRIMARY KEY,
  "name" VARCHAR(100) NOT NULL,
  "type" VARCHAR(20) NOT NULL,
  "target" TEXT NOT NULL,
  "interval_sec" INTEGER NOT NULL DEFAULT 60,
  "timeout_sec" INTEGER NOT NULL DEFAULT 5,
  "node_id" BIGINT,
  "enabled" INTEGER NOT NULL DEFAULT 1,
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "service_monitor_result" (
  "id" BIGSERIAL PRIMARY KEY,
  "monitor_id" BIGINT NOT NULL,
  "node_id" BIGINT NOT NULL,
  "timestamp" BIGINT NOT NULL,
  "success" INTEGER NOT NULL,
  "latency_ms" DOUBLE PRECISION,
  "status_code" INTEGER,
  "error_message" TEXT
);

CREATE TABLE IF NOT EXISTS "tunnel_quality" (
  "id" BIGSERIAL PRIMARY KEY,
  "tunnel_id" BIGINT NOT NULL,
  "entry_to_exit_latency" DOUBLE PRECISION,
  "exit_to_bing_latency" DOUBLE PRECISION,
  "entry_to_exit_loss" DOUBLE PRECISION,
  "exit_to_bing_loss" DOUBLE PRECISION,
  "success" INTEGER NOT NULL DEFAULT 1,
  "error_message" TEXT,
  "timestamp" BIGINT NOT NULL,
  "chain_details" TEXT
);

CREATE TABLE IF NOT EXISTS "forward_sla_state" (
  "forward_id" BIGINT PRIMARY KEY,
  "forward_name" VARCHAR(100) NOT NULL,
  "user_id" BIGINT NOT NULL,
  "tunnel_id" BIGINT NOT NULL,
  "mode" VARCHAR(20) NOT NULL,
  "overall_status" VARCHAR(20) NOT NULL,
  "entry_status" VARCHAR(20) NOT NULL,
  "target_status" VARCHAR(20) NOT NULL,
  "entry_total" INTEGER NOT NULL DEFAULT 0,
  "entry_healthy" INTEGER NOT NULL DEFAULT 0,
  "target_total" INTEGER NOT NULL DEFAULT 0,
  "target_healthy" INTEGER NOT NULL DEFAULT 0,
  "entry_checked_at" BIGINT NOT NULL DEFAULT 0,
  "target_checked_at" BIGINT NOT NULL DEFAULT 0,
  "checked_at" BIGINT NOT NULL,
  "uptime_24h" DOUBLE PRECISION NOT NULL DEFAULT 0,
  "samples_24h" INTEGER NOT NULL DEFAULT 0,
  "consecutive_failures" INTEGER NOT NULL DEFAULT 0,
  "first_failure_at" BIGINT NOT NULL DEFAULT 0,
  "last_failure_at" BIGINT NOT NULL DEFAULT 0,
  "last_healthy_at" BIGINT NOT NULL DEFAULT 0,
  "failure_kind" VARCHAR(40) NOT NULL DEFAULT '',
  "reason" TEXT NOT NULL DEFAULT '',
  "created_time" BIGINT NOT NULL,
  "updated_time" BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS "forward_sla_snapshot" (
  "id" BIGSERIAL PRIMARY KEY,
  "forward_id" BIGINT NOT NULL,
  "forward_name" VARCHAR(100) NOT NULL,
  "user_id" BIGINT NOT NULL,
  "tunnel_id" BIGINT NOT NULL,
  "mode" VARCHAR(20) NOT NULL,
  "overall_status" VARCHAR(20) NOT NULL,
  "entry_status" VARCHAR(20) NOT NULL,
  "target_status" VARCHAR(20) NOT NULL,
  "entry_total" INTEGER NOT NULL DEFAULT 0,
  "entry_healthy" INTEGER NOT NULL DEFAULT 0,
  "target_total" INTEGER NOT NULL DEFAULT 0,
  "target_healthy" INTEGER NOT NULL DEFAULT 0,
  "failure_kind" VARCHAR(40) NOT NULL DEFAULT '',
  "reason" TEXT NOT NULL DEFAULT '',
  "entry_checked_at" BIGINT NOT NULL DEFAULT 0,
  "target_checked_at" BIGINT NOT NULL DEFAULT 0,
  "timestamp" BIGINT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS "idx_user_tunnel_unique" ON "user_tunnel" ("user_id", "tunnel_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_tunnel_group_name" ON "tunnel_group" ("name");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_user_group_name" ON "user_group" ("name");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_tunnel_group_tunnel_unique" ON "tunnel_group_tunnel" ("tunnel_group_id", "tunnel_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_user_group_user_unique" ON "user_group_user" ("user_group_id", "user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_group_permission_unique" ON "group_permission" ("user_group_id", "tunnel_group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_group_permission_grant_unique" ON "group_permission_grant" ("user_group_id", "tunnel_group_id", "user_tunnel_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_monitor_permission_user" ON "monitor_permission" ("user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "uni_vite_config_name" ON "vite_config" ("name");
CREATE UNIQUE INDEX IF NOT EXISTS "uni_peer_share_token" ON "peer_share" ("token");
CREATE UNIQUE INDEX IF NOT EXISTS "uni_peer_share_runtime_reservation_id" ON "peer_share_runtime" ("reservation_id");
CREATE UNIQUE INDEX IF NOT EXISTS "uni_peer_share_runtime_resource_key" ON "peer_share_runtime" ("resource_key");
CREATE INDEX IF NOT EXISTS "idx_peer_share_runtime_binding_id" ON "peer_share_runtime" ("binding_id");
CREATE INDEX IF NOT EXISTS "idx_peer_share_runtime_share_node_status" ON "peer_share_runtime" ("share_id", "node_id", "status");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_federation_tunnel_binding_unique" ON "federation_tunnel_binding" ("tunnel_id", "node_id", "chain_type", "hop_inx");
CREATE UNIQUE INDEX IF NOT EXISTS "uni_federation_tunnel_binding_resource_key" ON "federation_tunnel_binding" ("resource_key");
CREATE INDEX IF NOT EXISTS "idx_federation_tunnel_binding_tunnel" ON "federation_tunnel_binding" ("tunnel_id", "status");
CREATE UNIQUE INDEX IF NOT EXISTS "uni_tunnel_cover_profile_tunnel_id" ON "tunnel_cover_profile" ("tunnel_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_cover_domain_profile_name" ON "cover_domain_profile" ("name");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_tunnel_cover_binding_unique" ON "tunnel_cover_binding" ("tunnel_id", "profile_id");
CREATE INDEX IF NOT EXISTS "idx_tunnel_cover_binding_tunnel" ON "tunnel_cover_binding" ("tunnel_id");
CREATE INDEX IF NOT EXISTS "idx_tunnel_cover_binding_profile" ON "tunnel_cover_binding" ("profile_id");
CREATE UNIQUE INDEX IF NOT EXISTS "uni_node_cover_service_node_id" ON "node_cover_service" ("node_id");
CREATE INDEX IF NOT EXISTS "idx_node_metric_node_time" ON "node_metric" ("node_id", "timestamp");
CREATE INDEX IF NOT EXISTS "idx_node_metric_time" ON "node_metric" ("timestamp");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_tunnel_metric_tunnel_time" ON "tunnel_metric" ("tunnel_id", "node_id", "timestamp");
CREATE UNIQUE INDEX IF NOT EXISTS "uidx_tunnel_metric_bucket" ON "tunnel_metric" ("tunnel_id", "node_id", "timestamp");
CREATE INDEX IF NOT EXISTS "idx_tunnel_metric_time" ON "tunnel_metric" ("timestamp");
CREATE INDEX IF NOT EXISTS "idx_service_monitor_node_id" ON "service_monitor" ("node_id");
CREATE INDEX IF NOT EXISTS "idx_monitor_result_monitor_time" ON "service_monitor_result" ("monitor_id", "timestamp");
CREATE INDEX IF NOT EXISTS "idx_service_monitor_result_node_id" ON "service_monitor_result" ("node_id");
CREATE INDEX IF NOT EXISTS "idx_tunnel_quality_tunnel_time" ON "tunnel_quality" ("tunnel_id", "timestamp");
CREATE INDEX IF NOT EXISTS "idx_tunnel_quality_time" ON "tunnel_quality" ("timestamp");
CREATE INDEX IF NOT EXISTS "idx_forward_sla_state_user_id" ON "forward_sla_state" ("user_id");
CREATE INDEX IF NOT EXISTS "idx_forward_sla_state_tunnel_id" ON "forward_sla_state" ("tunnel_id");
CREATE INDEX IF NOT EXISTS "idx_forward_sla_state_overall_status" ON "forward_sla_state" ("overall_status");
CREATE INDEX IF NOT EXISTS "idx_forward_sla_state_checked_at" ON "forward_sla_state" ("checked_at");
CREATE INDEX IF NOT EXISTS "idx_forward_sla_forward_time" ON "forward_sla_snapshot" ("forward_id", "timestamp");
CREATE INDEX IF NOT EXISTS "idx_forward_sla_snapshot_user_id" ON "forward_sla_snapshot" ("user_id");
CREATE INDEX IF NOT EXISTS "idx_forward_sla_snapshot_tunnel_id" ON "forward_sla_snapshot" ("tunnel_id");
CREATE INDEX IF NOT EXISTS "idx_forward_sla_snapshot_overall_status" ON "forward_sla_snapshot" ("overall_status");
CREATE INDEX IF NOT EXISTS "idx_forward_sla_time" ON "forward_sla_snapshot" ("timestamp");

COMMIT;
