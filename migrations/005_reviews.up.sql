CREATE TABLE reviews (
    id                   BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    repo_id              BIGINT UNSIGNED  NOT NULL,
    kind                 ENUM('review','audit')
                                          NOT NULL,
    status               ENUM('draft','open','changes_requested','accepted','resolved','closed')
                                          NOT NULL DEFAULT 'open'
                                          COMMENT 'Denormalized cache; recomputed by service layer',
    author_id            BIGINT UNSIGNED  NOT NULL,
    title                VARCHAR(512)     NOT NULL,
    summary              TEXT             NULL,
    origin_type          ENUM('manual','single_commit','commit_range','fork_change','similarity_cluster')
                                          NOT NULL,
    origin_payload       JSON             NULL,
    latest_revision_id   BIGINT UNSIGNED  NULL COMMENT 'FK added in migration 007',
    created_at           DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    closed_at            DATETIME         NULL,
    PRIMARY KEY (id),
    INDEX idx_reviews_repo_status (repo_id, status, updated_at),
    INDEX idx_reviews_author (author_id, created_at),
    CONSTRAINT fk_reviews_repo
        FOREIGN KEY (repo_id) REFERENCES repositories(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_reviews_author
        FOREIGN KEY (author_id) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
