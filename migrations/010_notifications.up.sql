CREATE TABLE notifications (
    id               BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    user_id          BIGINT UNSIGNED  NOT NULL,
    review_id        BIGINT UNSIGNED  NULL,
    comment_id       BIGINT UNSIGNED  NULL,
    event_type       VARCHAR(64)      NOT NULL,
    title            VARCHAR(512)     NOT NULL,
    body_preview     VARCHAR(512)     NULL,
    payload          JSON             NULL,
    is_read          BOOLEAN          NOT NULL DEFAULT FALSE,
    email_state      ENUM('not_applicable','pending','sent','failed','suppressed')
                                      NOT NULL DEFAULT 'not_applicable',
    email_attempts   TINYINT UNSIGNED NOT NULL DEFAULT 0,
    sent_at          DATETIME         NULL,
    read_at          DATETIME         NULL,
    created_at       DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_notif_user_read (user_id, is_read, created_at),
    INDEX idx_notif_review (review_id, created_at),
    INDEX idx_notif_email_state (email_state, created_at),
    CONSTRAINT fk_notif_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_notif_review
        FOREIGN KEY (review_id) REFERENCES reviews(id)
        ON DELETE SET NULL,
    CONSTRAINT fk_notif_comment
        FOREIGN KEY (comment_id) REFERENCES review_comments(id)
        ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notification_preferences (
    id               BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    user_id          BIGINT UNSIGNED  NOT NULL,
    event_type       VARCHAR(64)      NOT NULL COMMENT '* = default for all events',
    delivery_mode    ENUM('instant','digest','web_only')
                                      NOT NULL,
    suppress_self    BOOLEAN          NOT NULL DEFAULT TRUE,
    created_at       DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_notif_prefs (user_id, event_type),
    CONSTRAINT fk_notif_prefs_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE path_owner_rules (
    id               BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    repo_id          BIGINT UNSIGNED  NOT NULL,
    path_glob        VARCHAR(512)     NOT NULL,
    file_ext         VARCHAR(32)      NULL,
    target_user_id   BIGINT UNSIGNED  NOT NULL,
    action_role      ENUM('reviewer','blocking_reviewer','subscriber')
                                      NOT NULL,
    is_active        BOOLEAN          NOT NULL DEFAULT TRUE,
    created_by       BIGINT UNSIGNED  NOT NULL,
    created_at       DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_path_owners_repo (repo_id, is_active),
    INDEX idx_path_owners_target (target_user_id),
    CONSTRAINT fk_path_owners_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_path_owners_target
        FOREIGN KEY (target_user_id) REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_path_owners_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
