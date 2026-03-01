CREATE TABLE jobs (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    type              ENUM('clone','sync','fork_analysis')
                                       NOT NULL,
    status            ENUM('pending','running','completed','failed','cancelled')
                                       NOT NULL DEFAULT 'pending',
    repo_id           BIGINT UNSIGNED  NOT NULL,
    fork_id           BIGINT UNSIGNED  NULL,
    dedup_key         VARCHAR(255)     NULL COMMENT 'hash(type|repo_id|fork_id); NULL in terminal states',
    requested_by      BIGINT UNSIGNED  NOT NULL,
    progress_pct      TINYINT UNSIGNED NOT NULL DEFAULT 0,
    progress_message  VARCHAR(512)     NULL,
    result_summary    JSON             NULL,
    error_message     TEXT             NULL,
    retry_count       TINYINT UNSIGNED NOT NULL DEFAULT 0,
    max_retries       TINYINT UNSIGNED NOT NULL DEFAULT 3,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at        DATETIME         NULL,
    completed_at      DATETIME         NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_jobs_dedup (dedup_key),
    INDEX idx_jobs_repo_status (repo_id, status),
    INDEX idx_jobs_requested_by (requested_by),
    INDEX idx_jobs_status_created (status, created_at),
    CONSTRAINT fk_jobs_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_jobs_fork
        FOREIGN KEY (fork_id) REFERENCES forks(id)
        ON DELETE SET NULL,
    CONSTRAINT fk_jobs_requested_by
        FOREIGN KEY (requested_by) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
