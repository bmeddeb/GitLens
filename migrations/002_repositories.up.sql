CREATE TABLE repositories (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    github_id         BIGINT UNSIGNED  NOT NULL,
    owner             VARCHAR(255)     NOT NULL,
    name              VARCHAR(255)     NOT NULL,
    full_name         VARCHAR(512)     NOT NULL,
    description       TEXT             NULL,
    default_branch    VARCHAR(255)     NOT NULL DEFAULT 'main',
    clone_url         VARCHAR(512)     NOT NULL,
    is_fork           BOOLEAN          NOT NULL DEFAULT FALSE,
    is_private        BOOLEAN          NOT NULL DEFAULT FALSE,
    parent_github_id  BIGINT UNSIGNED  NULL,
    local_path        VARCHAR(512)     NULL,
    clone_status      ENUM('pending','cloning','ready','error')
                                       NOT NULL DEFAULT 'pending'
                                       COMMENT 'Denormalized from Job.status',
    last_synced_at    DATETIME         NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_repos_github_id (github_id),
    UNIQUE KEY uk_repos_full_name (full_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_repositories (
    id                       BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    user_id                  BIGINT UNSIGNED  NOT NULL,
    repo_id                  BIGINT UNSIGNED  NOT NULL,
    added_at                 DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    access_state             ENUM('active','suspended')
                                              NOT NULL DEFAULT 'active',
    last_access_verified_at  DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    nickname                 VARCHAR(255)     NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_repos (user_id, repo_id),
    INDEX idx_user_repos_repo_id (repo_id),
    CONSTRAINT fk_user_repos_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_user_repos_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
