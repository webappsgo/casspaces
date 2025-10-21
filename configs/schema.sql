-- ===============================
-- CasjayDev Workspaces Database Schema
-- Complete schema for all functionality
-- ===============================

-- Enable foreign key constraints (SQLite)
PRAGMA foreign_keys = ON;

-- Server configuration (single row, all server settings)
CREATE TABLE IF NOT EXISTS server_config (
    id INTEGER PRIMARY KEY DEFAULT 1,

    -- Server Identity
    server_identifier VARCHAR(255) NOT NULL DEFAULT 'casspaces',
    server_name VARCHAR(255) NOT NULL DEFAULT '',
    server_port INTEGER DEFAULT 0,
    server_title VARCHAR(255) NOT NULL DEFAULT 'CasjayDev Workspaces',

    -- SSL Configuration
    auto_ssl BOOLEAN DEFAULT FALSE,
    tls_cert_path VARCHAR(500),
    tls_key_path VARCHAR(500),
    admin_email VARCHAR(255),

    -- Reverse Proxy
    behind_proxy BOOLEAN DEFAULT FALSE,
    proxy_external_url VARCHAR(500),
    trust_proxy BOOLEAN DEFAULT FALSE,

    -- System Settings
    log_level VARCHAR(50) DEFAULT 'info',
    debug_mode BOOLEAN DEFAULT FALSE,
    show_detailed_status BOOLEAN DEFAULT FALSE,

    -- First run tracking
    setup_completed BOOLEAN DEFAULT FALSE,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (id = 1)
);

-- Runtime file paths (all configurable via UI)
CREATE TABLE IF NOT EXISTS runtime_paths (
    id INTEGER PRIMARY KEY DEFAULT 1,

    ssl_directory VARCHAR(500) DEFAULT '/etc/casspaces/ssl',
    security_directory VARCHAR(500) DEFAULT '/etc/casspaces/security',
    log_directory VARCHAR(500) DEFAULT '/var/log/casspaces',
    backup_directory VARCHAR(500) DEFAULT '/mnt/Backups',
    cache_directory VARCHAR(500) DEFAULT '/var/lib/casspaces/cache',
    data_directory VARCHAR(500) DEFAULT '/var/lib/casspaces',
    pid_directory VARCHAR(500) DEFAULT '/var/run/casspaces',

    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (id = 1)
);

-- Security configuration (mandatory features hardcoded, optional configurable)
CREATE TABLE IF NOT EXISTS security_config (
    id INTEGER PRIMARY KEY DEFAULT 1,

    -- Security Data Updates (1-48 hours, configurable)
    security_update_frequency_hours INTEGER DEFAULT 24 CHECK (
        security_update_frequency_hours >= 1 AND security_update_frequency_hours <= 48
    ),
    last_security_update TIMESTAMP,

    -- Geographic Protection (always monitoring, blocking configurable)
    geo_blocking_mode VARCHAR(20) DEFAULT 'monitor' CHECK (
        geo_blocking_mode IN ('monitor', 'none', 'block_high_risk', 'block_custom')
    ),
    blocked_countries TEXT DEFAULT '[]',
    allowed_countries TEXT DEFAULT '[]',
    custom_high_risk_countries TEXT DEFAULT '[]',

    -- Optional Security Features (configurable)
    rate_limiting_enabled BOOLEAN DEFAULT TRUE,
    captcha_enabled BOOLEAN DEFAULT FALSE,
    two_factor_required BOOLEAN DEFAULT FALSE,
    ip_whitelist_enabled BOOLEAN DEFAULT FALSE,

    -- Feature Configuration
    requests_per_minute INTEGER DEFAULT 300,
    requests_per_hour INTEGER DEFAULT 5000,
    max_failed_logins INTEGER DEFAULT 5,
    lockout_duration_minutes INTEGER DEFAULT 15,
    max_concurrent_sessions INTEGER DEFAULT 5,
    idle_timeout_minutes INTEGER DEFAULT 30,

    -- IP Access Control (JSON arrays)
    ip_whitelist TEXT DEFAULT '[]',
    ip_blacklist TEXT DEFAULT '[]',

    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (id = 1)
);

-- User management
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255),

    -- User status
    active BOOLEAN DEFAULT TRUE,
    is_admin BOOLEAN DEFAULT FALSE,
    is_temp_admin BOOLEAN DEFAULT FALSE,
    email_verified BOOLEAN DEFAULT FALSE,

    -- Security tracking
    failed_login_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP,
    last_login TIMESTAMP,
    last_login_ip VARCHAR(45),
    password_changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- 2FA
    two_factor_enabled BOOLEAN DEFAULT FALSE,
    two_factor_secret VARCHAR(255),
    two_factor_backup_codes TEXT, -- JSON array

    -- Quotas and limits
    workspace_quota INTEGER DEFAULT 10,
    storage_quota_gb INTEGER DEFAULT 10,

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- User sessions (for session management)
CREATE TABLE IF NOT EXISTS user_sessions (
    id VARCHAR(255) PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip_address VARCHAR(45),
    user_agent TEXT,
    active BOOLEAN DEFAULT TRUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Workspaces (user workspace instances)
CREATE TABLE IF NOT EXISTS workspaces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    image VARCHAR(500) NOT NULL,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Workspace state
    status VARCHAR(50) DEFAULT 'stopped', -- stopped, starting, running, stopping, error
    container_id VARCHAR(255),
    container_name VARCHAR(255),

    -- Resource allocation
    cpu_limit DECIMAL(3,1),
    memory_limit_gb INTEGER,
    storage_limit_gb INTEGER,

    -- Network configuration
    vnc_port INTEGER,
    spice_port INTEGER,
    vnc_password VARCHAR(255),
    websocket_port INTEGER,

    -- Storage
    persistent_storage BOOLEAN DEFAULT TRUE,
    storage_path VARCHAR(500),

    -- Lifecycle management
    auto_stop_hours INTEGER DEFAULT 8,
    last_activity TIMESTAMP,
    started_at TIMESTAMP,
    stopped_at TIMESTAMP,

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Audit logging (mandatory - cannot be disabled)
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    user_id INTEGER REFERENCES users(id),
    username VARCHAR(255),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    ip_address VARCHAR(45),
    user_agent TEXT,
    result VARCHAR(50), -- 'success', 'failure', 'denied'
    details TEXT, -- JSON additional details
    compliance_relevant BOOLEAN DEFAULT FALSE
);

-- Geographic access log (mandatory - always populated)
CREATE TABLE IF NOT EXISTS geographic_access_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ip_address VARCHAR(45) NOT NULL,
    country_code VARCHAR(2),
    country_name VARCHAR(100),
    region VARCHAR(100),
    city VARCHAR(100),
    latitude DECIMAL(10,8),
    longitude DECIMAL(11,8),
    blocked BOOLEAN DEFAULT FALSE,
    block_reason VARCHAR(100),
    user_id INTEGER REFERENCES users(id),
    username VARCHAR(255),
    request_path VARCHAR(500),
    request_method VARCHAR(10),
    user_agent TEXT,
    response_status INTEGER
);

-- Security events log (mandatory)
CREATE TABLE IF NOT EXISTS security_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    event_type VARCHAR(100) NOT NULL, -- threat_blocked, vulnerability_detected, etc.
    severity VARCHAR(20) NOT NULL, -- low, medium, high, critical
    source_ip VARCHAR(45),
    target_ip VARCHAR(45),
    user_id INTEGER REFERENCES users(id),
    username VARCHAR(255),
    description TEXT NOT NULL,
    details TEXT, -- JSON additional event data
    resolved BOOLEAN DEFAULT FALSE,
    resolved_at TIMESTAMP,
    resolved_by INTEGER REFERENCES users(id),
    resolution_notes TEXT
);

-- Rate limiting tracking
CREATE TABLE IF NOT EXISTS rate_limits (
    id VARCHAR(255) PRIMARY KEY, -- IP address or user ID
    limit_type VARCHAR(50) NOT NULL, -- 'ip', 'user'
    requests_minute INTEGER DEFAULT 0,
    requests_hour INTEGER DEFAULT 0,
    requests_day INTEGER DEFAULT 0,
    last_request TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    blocked_until TIMESTAMP,
    total_blocked INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert default configurations
INSERT OR IGNORE INTO server_config (id) VALUES (1);
INSERT OR IGNORE INTO runtime_paths (id) VALUES (1);
INSERT OR IGNORE INTO security_config (id) VALUES (1);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_workspaces_user_id ON workspaces(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_security_events_timestamp ON security_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_geo_access_timestamp ON geographic_access_log(timestamp);