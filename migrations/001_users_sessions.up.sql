CREATE TABLE users (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    github_id         BIGINT UNSIGNED  NOT NULL,
    username          VARCHAR(255)     NOT NULL,
    display_name      VARCHAR(255)     NULL,
    email             VARCHAR(255)     NULL,
    avatar_url        VARCHAR(512)     NULL,
    access_token      VARBINARY(512)   NOT NULL COMMENT 'AES-256-GCM encrypted',
    token_expiry      DATETIME         NULL,
    role              ENUM('user','admin') NOT NULL DEFAULT 'user',
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    last_login_at     DATETIME         NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_users_github_id (github_id),
    UNIQUE KEY uk_users_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE sessions (
    id                CHAR(64)         NOT NULL COMMENT 'Cryptographic random hex',
    user_id           BIGINT UNSIGNED  NOT NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at        DATETIME         NOT NULL,
    last_active_at    DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_sessions_user_id (user_id),
    INDEX idx_sessions_expires_at (expires_at),
    CONSTRAINT fk_sessions_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
