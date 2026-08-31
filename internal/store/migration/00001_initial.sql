-- +goose Up
-- Plume schema: initial baseline.
--
-- This is the full schema applied on first boot. Every statement is guarded
-- with IF NOT EXISTS / IF EXISTS so re-running it is a safe no-op.
--
-- A few choices worth calling out:
--   * user_presence is keyed by (user_id, org_id), not just user_id. A person
--     can belong to several organizations and their status is tracked per
--     workspace, so a single global row would let joining one clobber another.
--   * pending_attachments.uploaded_by records who uploaded the file. Pending
--     uploads exist before the message that references them, so we keep the
--     owner to stop one user attaching someone else's upload.
--   * idx_notifications_unread is a partial index over is_read = 0. The unread
--     badge and the unread-only list both filter on is_read = 0, so only that
--     slice is worth indexing.

-- Core identity & org tables ---------------------------------------------------
CREATE TABLE IF NOT EXISTS organizations (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    message_edit_window_minutes INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS accounts (
    id            TEXT PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    account_id TEXT,
    org_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    avatar_url TEXT,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(org_id, email)
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member',
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    user_agent TEXT,
    ip_address TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS password_resets (
    id          TEXT PRIMARY KEY,
    account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL,
    expires_at  TEXT NOT NULL,
    used_at     TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS user_preferences (
    user_id              TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    theme                TEXT NOT NULL DEFAULT 'system',
    language             TEXT NOT NULL DEFAULT 'en',
    timezone             TEXT NOT NULL DEFAULT 'UTC',
    email_notifications  INTEGER NOT NULL DEFAULT 1,
    desktop_notifications INTEGER NOT NULL DEFAULT 1,
    notification_sounds  INTEGER NOT NULL DEFAULT 1,
    sidebar_collapsed    INTEGER NOT NULL DEFAULT 0,
    motion_settings      TEXT NOT NULL DEFAULT '{}',
    created_at           TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Presence is tracked per organization membership, so the key spans
-- (user_id, org_id) rather than user_id alone.
CREATE TABLE IF NOT EXISTS user_presence (
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'offline' CHECK(status IN ('online','away','offline','dnd')),
    last_seen   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, org_id)
);

CREATE TABLE IF NOT EXISTS labels (
    id         TEXT PRIMARY KEY,
    org_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    color      TEXT NOT NULL DEFAULT '#6366f1',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_accounts_email ON accounts(email);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_password_resets_account_id ON password_resets(account_id);
CREATE INDEX IF NOT EXISTS idx_password_resets_token_hash ON password_resets(token_hash);
CREATE INDEX IF NOT EXISTS idx_user_presence_org ON user_presence(org_id, status);
CREATE INDEX IF NOT EXISTS idx_users_account_id ON users(account_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(org_id, email);
CREATE INDEX IF NOT EXISTS idx_users_org_id ON users(org_id);
CREATE INDEX IF NOT EXISTS idx_labels_org_id ON labels(org_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_labels_org_name ON labels(org_id, name);

-- Project tables --------------------------------------------------------------
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    org_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    slug TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT 'oklch(0.6 0.15 250)',
    icon TEXT NOT NULL DEFAULT 'FolderIcon',
    created_by TEXT NOT NULL REFERENCES users(id),
    cycle_duration INTEGER,
    auto_generate_cycles BOOLEAN NOT NULL DEFAULT FALSE,
    incomplete_task_handling TEXT NOT NULL DEFAULT 'next_cycle'
        CHECK(incomplete_task_handling IN ('next_cycle','backlog')),
    starts_at TEXT,
    ends_at TEXT,
    is_archived BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(org_id, slug)
);

CREATE TABLE IF NOT EXISTS project_members (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('admin', 'member', 'viewer', 'guest')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (project_id, user_id)
);

CREATE TABLE IF NOT EXISTS cycles (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    org_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    goal TEXT NOT NULL DEFAULT '',
    starts_at TEXT NOT NULL,
    ends_at TEXT NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(id),
    is_completed BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS task_statuses (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    color TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    category TEXT NOT NULL DEFAULT 'todo' CHECK(category IN ('todo','in_progress','done','canceled')),
    is_default BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS task_templates (
    id                 TEXT PRIMARY KEY,
    org_id             TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id         TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name               TEXT NOT NULL,
    description        TEXT NOT NULL DEFAULT '',
    priority           TEXT NOT NULL DEFAULT 'none',
    status_id          TEXT NOT NULL REFERENCES task_statuses(id) ON DELETE CASCADE,
    assignee_ids       TEXT NOT NULL DEFAULT '[]',
    estimate           INTEGER,
    recurrence_pattern TEXT NOT NULL DEFAULT 'none'
        CHECK(recurrence_pattern IN ('none', 'daily', 'weekly', 'monthly')),
    recurrence_days    TEXT NOT NULL DEFAULT '',
    next_run_at        TEXT,
    last_error         TEXT,
    last_error_at      TEXT,
    created_by         TEXT NOT NULL REFERENCES users(id),
    created_at         TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS custom_fields (
    id          TEXT PRIMARY KEY,
    org_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    field_type  TEXT NOT NULL DEFAULT 'text'
        CHECK(field_type IN ('text', 'number', 'select', 'date')),
    options     TEXT NOT NULL DEFAULT '[]',
    position    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    org_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    cycle_id TEXT REFERENCES cycles(id) ON DELETE SET NULL,
    parent_task_id TEXT REFERENCES tasks(id) ON DELETE SET NULL,
    created_by TEXT NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status_id TEXT NOT NULL REFERENCES task_statuses(id) ON DELETE RESTRICT,
    priority TEXT NOT NULL DEFAULT 'none'
        CHECK(priority IN ('none','low','medium','high','urgent')),
    estimate INTEGER,
    position_key TEXT NOT NULL DEFAULT '',
    subtask_position TEXT,
    template_id TEXT REFERENCES task_templates(id) ON DELETE SET NULL,
    started_at TEXT,
    due_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS task_assignees (
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, user_id)
);

CREATE TABLE IF NOT EXISTS task_labels (
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    label_id TEXT NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, label_id)
);

CREATE TABLE IF NOT EXISTS task_dependencies (
    task_id       TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    blocks_task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (task_id, blocks_task_id),
    CHECK (task_id <> blocks_task_id)
);

CREATE TABLE IF NOT EXISTS task_custom_field_values (
    task_id      TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    custom_field_id TEXT NOT NULL REFERENCES custom_fields(id) ON DELETE CASCADE,
    value        TEXT NOT NULL DEFAULT '',
    updated_at   TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (task_id, custom_field_id)
);

CREATE TABLE IF NOT EXISTS task_activity (
    id         TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    org_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    actor_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action     TEXT NOT NULL,
    field      TEXT NOT NULL DEFAULT '',
    old_value  TEXT NOT NULL DEFAULT '',
    new_value  TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS attachments (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size INTEGER NOT NULL,
    storage_path TEXT NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS time_entries (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    description TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    ended_at TEXT,
    duration_minutes INTEGER,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS comments (
    id         TEXT PRIMARY KEY,
    org_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    author_id  TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT NOT NULL,
    parent_id  TEXT REFERENCES comments(id),
    deleted_at TEXT,
    edited_at  TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS views (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    org_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    layout TEXT NOT NULL DEFAULT 'board' CHECK(layout IN ('board','list')),
    filters TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(org_id, project_id, name)
);

CREATE TABLE IF NOT EXISTS view_pins (
    view_id TEXT NOT NULL REFERENCES views(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (view_id, user_id)
);

CREATE TABLE IF NOT EXISTS dashboard_preferences (
    user_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id    TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    sections  TEXT NOT NULL DEFAULT '["my_tasks","due_soon","activity","stats","projects"]',
    PRIMARY KEY (user_id, org_id)
);

CREATE TABLE IF NOT EXISTS audit_log (
    id         TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    org_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    actor_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action     TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id  TEXT NOT NULL DEFAULT '',
    metadata   TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_projects_org_id ON projects(org_id);
CREATE INDEX IF NOT EXISTS idx_projects_archived ON projects(org_id, is_archived);
CREATE INDEX IF NOT EXISTS idx_projects_slug ON projects(org_id, slug);
CREATE INDEX IF NOT EXISTS idx_project_members_project ON project_members(project_id);
CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id);
CREATE INDEX IF NOT EXISTS idx_cycles_project ON cycles(project_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cycles_active ON cycles(project_id) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_cycle ON tasks(cycle_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status_id);
CREATE INDEX IF NOT EXISTS idx_tasks_position_key ON tasks(project_id, status_id, position_key);
CREATE INDEX IF NOT EXISTS idx_tasks_template_id ON tasks(template_id);
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_task_id) WHERE parent_task_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_task_assignees_task ON task_assignees(task_id);
CREATE INDEX IF NOT EXISTS idx_task_assignees_user ON task_assignees(user_id);
CREATE INDEX IF NOT EXISTS idx_task_labels_task_id ON task_labels(task_id);
CREATE INDEX IF NOT EXISTS idx_task_labels_label_id ON task_labels(label_id);
CREATE INDEX IF NOT EXISTS idx_task_deps_task_id ON task_dependencies(task_id);
CREATE INDEX IF NOT EXISTS idx_task_deps_blocks_id ON task_dependencies(blocks_task_id);
CREATE INDEX IF NOT EXISTS idx_task_cf_values_task ON task_custom_field_values(task_id);
CREATE INDEX IF NOT EXISTS idx_task_activity_task ON task_activity(task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_activity_project ON task_activity(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_attachments_task ON attachments(task_id);
CREATE INDEX IF NOT EXISTS idx_time_entries_task ON time_entries(task_id);
CREATE INDEX IF NOT EXISTS idx_time_entries_user ON time_entries(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_time_entries_active_user ON time_entries(user_id) WHERE ended_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_comments_task_id ON comments(task_id);
CREATE INDEX IF NOT EXISTS idx_comments_project_id ON comments(project_id);
CREATE INDEX IF NOT EXISTS idx_comments_author_id ON comments(author_id);
CREATE INDEX IF NOT EXISTS idx_comments_parent_id ON comments(parent_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_statuses_name_unique ON task_statuses(project_id, name);
CREATE INDEX IF NOT EXISTS idx_task_statuses_project ON task_statuses(project_id);
CREATE INDEX IF NOT EXISTS idx_task_templates_org ON task_templates(org_id);
CREATE INDEX IF NOT EXISTS idx_task_templates_project ON task_templates(project_id);
CREATE INDEX IF NOT EXISTS idx_task_templates_created_by ON task_templates(created_by);
CREATE INDEX IF NOT EXISTS idx_task_templates_next_run ON task_templates(next_run_at) WHERE next_run_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_custom_fields_project ON custom_fields(project_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_org_id ON audit_log(org_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(org_id, created_at DESC);

-- Composite org+project indexes
CREATE INDEX IF NOT EXISTS idx_tasks_org_project ON tasks(org_id, project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_org_project_status_position
    ON tasks(org_id, project_id, status_id, position_key);

-- Chat tables ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS conversations (
    id          TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    org_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    parent_id   TEXT REFERENCES conversations(id) ON DELETE SET NULL,
    name        TEXT NOT NULL DEFAULT '',
    topic       TEXT NOT NULL DEFAULT '',
    type        TEXT NOT NULL CHECK(type IN ('direct', 'group', 'channel', 'voice', 'category')),
    created_by  TEXT NOT NULL REFERENCES users(id),
    position_key TEXT NOT NULL DEFAULT 'h',
    deleted_at  TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS conversation_members (
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id          TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    joined_at       TEXT NOT NULL DEFAULT (datetime('now')),
    last_read_at    TEXT NOT NULL DEFAULT (datetime('now')),
    muted           INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE IF NOT EXISTS messages (
    id                   TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    conversation_id      TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    org_id               TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    sender_id            TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content              TEXT NOT NULL DEFAULT '',
    search_content       TEXT NOT NULL DEFAULT '',
    parent_id            TEXT REFERENCES messages(id) ON DELETE SET NULL,
    forwarded_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL,
    pinned               INTEGER NOT NULL DEFAULT 0,
    pinned_at            TEXT,
    pinned_by            TEXT REFERENCES users(id),
    edited_at            TEXT,
    deleted_at           TEXT,
    created_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS message_attachments (
    id           TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    message_id   TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    file_name    TEXT NOT NULL,
    file_size    INTEGER NOT NULL,
    content_type TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS message_reactions (
    message_id   TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id       TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    emoji        TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (message_id, user_id, emoji)
);

CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    message_id UNINDEXED,
    org_id UNINDEXED,
    conversation_id UNINDEXED,
    content,
    attachment_names,
    tokenize='trigram'
);

CREATE TABLE IF NOT EXISTS pending_attachments (
    id              TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    file_name       TEXT NOT NULL,
    file_size       INTEGER NOT NULL,
    content_type    TEXT NOT NULL,
    storage_path    TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    uploaded_by     TEXT NOT NULL DEFAULT '' REFERENCES users(id) ON DELETE CASCADE -- who uploaded it; claimed when the upload is attached to a message
);

CREATE TABLE IF NOT EXISTS channel_permissions (
    channel_id  TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role        TEXT NOT NULL CHECK(role IN ('everyone', 'owner', 'admin', 'member', 'viewer', 'guest')),
    permission  TEXT NOT NULL CHECK(permission IN ('channel:view', 'channel:send', 'channel:manage', 'channel:permissions')),
    allow       INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (channel_id, role, permission)
);

CREATE TABLE IF NOT EXISTS channel_user_overrides (
    channel_id  TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission  TEXT NOT NULL CHECK(permission IN ('channel:view', 'channel:send', 'channel:manage', 'channel:permissions')),
    allow       INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (channel_id, user_id, permission)
);

CREATE TABLE IF NOT EXISTS channel_project_links (
    channel_id  TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    PRIMARY KEY (channel_id, project_id)
);

CREATE TABLE IF NOT EXISTS user_channel_preferences (
    user_id             TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id     TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    org_id              TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    notification_level  TEXT NOT NULL DEFAULT 'mentions'
        CHECK(notification_level IN ('all', 'mentions', 'nothing')),
    muted               INTEGER NOT NULL DEFAULT 0,
    last_read_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, conversation_id)
);

CREATE TABLE IF NOT EXISTS voice_participants (
    id              TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    org_id          TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    muted           INTEGER NOT NULL DEFAULT 0,
    deafened        INTEGER NOT NULL DEFAULT 0,
    connection_id   TEXT NOT NULL DEFAULT '',
    joined_at       TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(conversation_id, user_id)
);

CREATE TABLE IF NOT EXISTS user_invites (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    org_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email TEXT,
    role TEXT NOT NULL DEFAULT 'member'
        CHECK(role IN ('admin', 'member', 'viewer', 'guest')),
    token_hash TEXT NOT NULL,
    invited_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    max_uses INTEGER,
    use_count INTEGER NOT NULL DEFAULT 0,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS user_invite_acceptances (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    invite_id TEXT NOT NULL REFERENCES user_invites(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS invite_projects (
    invite_id TEXT NOT NULL REFERENCES user_invites(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('admin', 'member', 'viewer', 'guest')),
    PRIMARY KEY (invite_id, project_id)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_conversations_org ON conversations(org_id);
CREATE INDEX IF NOT EXISTS idx_conversations_parent ON conversations(parent_id, position_key);
CREATE INDEX IF NOT EXISTS idx_conversations_active ON conversations(org_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_conversations_active_ordered
    ON conversations(org_id, position_key, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_conv_members_user ON conversation_members(user_id, org_id);
CREATE INDEX IF NOT EXISTS idx_conversation_members_user_id ON conversation_members(user_id, conversation_id);
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_messages_org_deleted_created ON messages(org_id, deleted_at, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_messages_parent ON messages(conversation_id, parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_pinned_by ON messages(pinned_by);
CREATE INDEX IF NOT EXISTS idx_message_attachments_message_id ON message_attachments(message_id);
CREATE INDEX IF NOT EXISTS idx_msg_attachments_message ON message_attachments(message_id);
CREATE INDEX IF NOT EXISTS idx_message_reactions_message ON message_reactions(message_id);
CREATE INDEX IF NOT EXISTS idx_message_reactions_user ON message_reactions(user_id, org_id);
CREATE INDEX IF NOT EXISTS idx_pending_attachments_created ON pending_attachments(created_at);
CREATE INDEX IF NOT EXISTS idx_channel_permissions_channel ON channel_permissions(channel_id);
CREATE INDEX IF NOT EXISTS idx_channel_user_overrides_channel ON channel_user_overrides(channel_id);
CREATE INDEX IF NOT EXISTS idx_channel_user_overrides_user ON channel_user_overrides(user_id);
CREATE INDEX IF NOT EXISTS idx_channel_project_links_project ON channel_project_links(project_id);
CREATE INDEX IF NOT EXISTS idx_user_channel_prefs_conv ON user_channel_preferences(conversation_id, user_id);
CREATE INDEX IF NOT EXISTS idx_voice_participants_conv ON voice_participants(conversation_id);
CREATE INDEX IF NOT EXISTS idx_voice_participants_org ON voice_participants(org_id);
CREATE INDEX IF NOT EXISTS idx_user_invites_org ON user_invites(org_id);
CREATE INDEX IF NOT EXISTS idx_user_invites_token ON user_invites(token_hash);
CREATE INDEX IF NOT EXISTS idx_invite_projects_invite ON invite_projects(invite_id);

-- Triggers for message FTS sync
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts(message_id, org_id, conversation_id, content, attachment_names)
    VALUES (NEW.id, NEW.org_id, NEW.conversation_id, NEW.search_content, '');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
    DELETE FROM messages_fts WHERE message_id = OLD.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
    DELETE FROM messages_fts WHERE message_id = OLD.id;
    INSERT INTO messages_fts(message_id, org_id, conversation_id, content, attachment_names)
    VALUES (
        NEW.id, NEW.org_id, NEW.conversation_id, NEW.search_content,
        COALESCE((SELECT group_concat(file_name, ' ') FROM message_attachments WHERE message_id = NEW.id), '')
    );
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS message_attachments_ai AFTER INSERT ON message_attachments BEGIN
    UPDATE messages_fts SET attachment_names = COALESCE((
        SELECT group_concat(file_name, ' ') FROM message_attachments WHERE message_id = NEW.message_id
    ), '') WHERE message_id = NEW.message_id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS message_attachments_ad AFTER DELETE ON message_attachments BEGIN
    UPDATE messages_fts SET attachment_names = COALESCE((
        SELECT group_concat(file_name, ' ') FROM message_attachments WHERE message_id = OLD.message_id
    ), '') WHERE message_id = OLD.message_id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS message_attachments_au AFTER UPDATE ON message_attachments BEGIN
    UPDATE messages_fts SET attachment_names = COALESCE((
        SELECT group_concat(file_name, ' ') FROM message_attachments WHERE message_id = NEW.message_id
    ), '') WHERE message_id = NEW.message_id;
END;
-- +goose StatementEnd

-- Notifications ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS notifications (
    id          TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    org_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL DEFAULT '',
    link        TEXT NOT NULL DEFAULT '',
    entity_type TEXT NOT NULL DEFAULT '',
    entity_id   TEXT NOT NULL DEFAULT '',
    actor_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    is_read     INTEGER NOT NULL DEFAULT 0,
    read_at     TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS notification_preferences (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type    TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (user_id, type)
);

CREATE TABLE IF NOT EXISTS push_subscriptions (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    endpoint    TEXT NOT NULL,
    p256dh      TEXT NOT NULL,
    auth_key    TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (user_id, endpoint)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_notifications_user_cursor
    ON notifications(user_id, org_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_entity ON notifications(entity_type, entity_id);
-- Partial index over only unread rows (is_read = 0); backs the unread badge.
CREATE INDEX IF NOT EXISTS idx_notifications_unread
    ON notifications(user_id, is_read) WHERE is_read = 0;
CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user ON push_subscriptions(user_id);

-- FTS5 search tables -----------------------------------------------------------
CREATE VIRTUAL TABLE IF NOT EXISTS tasks_fts USING fts5(
    task_id UNINDEXED,
    org_id UNINDEXED,
    project_id UNINDEXED,
    title,
    description,
    tokenize='trigram'
);

CREATE VIRTUAL TABLE IF NOT EXISTS projects_fts USING fts5(
    project_id UNINDEXED,
    org_id UNINDEXED,
    name,
    slug,
    tokenize='trigram'
);

-- Tasks FTS sync triggers
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS tasks_fts_ai AFTER INSERT ON tasks BEGIN
    INSERT INTO tasks_fts(task_id, org_id, project_id, title, description)
    VALUES (NEW.id, NEW.org_id, NEW.project_id, NEW.title, NEW.description);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS tasks_fts_ad AFTER DELETE ON tasks BEGIN
    DELETE FROM tasks_fts WHERE task_id = OLD.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS tasks_fts_au AFTER UPDATE ON tasks BEGIN
    DELETE FROM tasks_fts WHERE task_id = OLD.id;
    INSERT INTO tasks_fts(task_id, org_id, project_id, title, description)
    VALUES (NEW.id, NEW.org_id, NEW.project_id, NEW.title, NEW.description);
END;
-- +goose StatementEnd

-- Projects FTS sync triggers
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS projects_fts_ai AFTER INSERT ON projects BEGIN
    INSERT INTO projects_fts(project_id, org_id, name, slug)
    VALUES (NEW.id, NEW.org_id, NEW.name, NEW.slug);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS projects_fts_ad AFTER DELETE ON projects BEGIN
    DELETE FROM projects_fts WHERE project_id = OLD.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS projects_fts_au AFTER UPDATE ON projects BEGIN
    DELETE FROM projects_fts WHERE project_id = OLD.id;
    INSERT INTO projects_fts(project_id, org_id, name, slug)
    VALUES (NEW.id, NEW.org_id, NEW.name, NEW.slug);
END;
-- +goose StatementEnd

-- Seed the search indexes from any existing rows. On a fresh database this is
-- a no-op, but it keeps the FTS tables in sync if the schema is ever rebuilt.
INSERT OR IGNORE INTO tasks_fts(task_id, org_id, project_id, title, description)
    SELECT id, org_id, project_id, title, description FROM tasks;
INSERT OR IGNORE INTO projects_fts(project_id, org_id, name, slug)
    SELECT id, org_id, name, slug FROM projects;
