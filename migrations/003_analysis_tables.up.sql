CREATE TABLE commits (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    repo_id           BIGINT UNSIGNED  NOT NULL,
    sha               CHAR(40)         NOT NULL,
    author_name       VARCHAR(255)     NULL,
    author_email      VARCHAR(255)     NULL,
    committer_name    VARCHAR(255)     NULL,
    committed_at      DATETIME         NOT NULL,
    message           TEXT             NULL,
    files_changed     INT UNSIGNED     NULL,
    insertions        INT UNSIGNED     NULL,
    deletions         INT UNSIGNED     NULL,
    parent_shas       JSON             NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_commits_repo_sha (repo_id, sha),
    INDEX idx_commits_repo_date (repo_id, committed_at DESC),
    INDEX idx_commits_repo_author (repo_id, author_email),
    CONSTRAINT fk_commits_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE contributor_identities (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    canonical_email   VARCHAR(255)     NOT NULL,
    display_name      VARCHAR(255)     NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_contrib_identity_email (canonical_email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE contributor_aliases (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    identity_id       BIGINT UNSIGNED  NOT NULL,
    observed_name     VARCHAR(255)     NULL,
    observed_email    VARCHAR(255)     NOT NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_contrib_alias_email (observed_email),
    INDEX idx_contrib_alias_identity (identity_id),
    CONSTRAINT fk_contrib_alias_identity
        FOREIGN KEY (identity_id) REFERENCES contributor_identities(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE contributor_stats (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    repo_id           BIGINT UNSIGNED  NOT NULL,
    identity_id       BIGINT UNSIGNED  NOT NULL,
    commits_count     INT UNSIGNED     NOT NULL DEFAULT 0,
    insertions        BIGINT UNSIGNED  NOT NULL DEFAULT 0,
    deletions         BIGINT UNSIGNED  NOT NULL DEFAULT 0,
    first_commit_at   DATETIME         NULL,
    last_commit_at    DATETIME         NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_contrib_stats (repo_id, identity_id),
    CONSTRAINT fk_contrib_stats_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_contrib_stats_identity
        FOREIGN KEY (identity_id) REFERENCES contributor_identities(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE forks (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    source_repo_id    BIGINT UNSIGNED  NOT NULL,
    github_fork_id    BIGINT UNSIGNED  NOT NULL,
    fork_owner        VARCHAR(255)     NOT NULL,
    fork_name         VARCHAR(255)     NOT NULL,
    fork_full_name    VARCHAR(512)     NOT NULL,
    clone_url         VARCHAR(512)     NULL,
    local_path        VARCHAR(512)     NULL,
    ahead_by          INT UNSIGNED     NOT NULL DEFAULT 0,
    behind_by         INT UNSIGNED     NOT NULL DEFAULT 0,
    merge_base_sha    CHAR(40)         NULL,
    diverged_at       DATETIME         NULL,
    analysis_status   ENUM('listed','pending','analyzing','complete','error')
                                       NOT NULL DEFAULT 'listed'
                                       COMMENT 'Denormalized from Job.status',
    last_analyzed_at  DATETIME         NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_forks_github_id (github_fork_id),
    INDEX idx_forks_source_repo (source_repo_id),
    CONSTRAINT fk_forks_source_repo
        FOREIGN KEY (source_repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE fork_changes (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    fork_id           BIGINT UNSIGNED  NOT NULL,
    commit_sha        CHAR(40)         NOT NULL,
    file_path         VARCHAR(1024)    NULL,
    change_type       ENUM('feature','bugfix','refactor','docs','chore','unknown')
                                       NOT NULL DEFAULT 'unknown',
    confidence        DECIMAL(3,2)     NOT NULL DEFAULT 0.30,
    is_cherry_pick    BOOLEAN          NOT NULL DEFAULT FALSE,
    is_rebase         BOOLEAN          NOT NULL DEFAULT FALSE,
    upstream_sha      CHAR(40)         NULL COMMENT 'Matched upstream commit if cherry-pick/rebase',
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_fork_changes (fork_id, commit_sha),
    INDEX idx_fork_changes_type (fork_id, change_type),
    CONSTRAINT fk_fork_changes_fork
        FOREIGN KEY (fork_id) REFERENCES forks(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE blame_cache (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    repo_id           BIGINT UNSIGNED  NOT NULL,
    file_path         VARCHAR(1024)    NOT NULL,
    commit_sha        CHAR(40)         NOT NULL,
    blame_data        MEDIUMBLOB       NOT NULL COMMENT 'Gzip compressed, max 5 MB uncompressed',
    line_count        INT UNSIGNED     NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_blame_cache (repo_id, file_path(255), commit_sha),
    INDEX idx_blame_cache_created (created_at),
    CONSTRAINT fk_blame_cache_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
    ROW_FORMAT=COMPRESSED;
