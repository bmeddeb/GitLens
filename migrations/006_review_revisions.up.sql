CREATE TABLE review_revisions (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    review_id         BIGINT UNSIGNED  NOT NULL,
    revision_number   INT UNSIGNED     NOT NULL,
    base_ref          VARCHAR(255)     NULL,
    head_ref          VARCHAR(255)     NULL,
    base_commit_sha   CHAR(40)         NOT NULL,
    head_commit_sha   CHAR(40)         NOT NULL,
    file_count        INT UNSIGNED     NULL,
    insertions        INT UNSIGNED     NULL,
    deletions         INT UNSIGNED     NULL,
    created_by        BIGINT UNSIGNED  NOT NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_revisions_review_num (review_id, revision_number),
    INDEX idx_revisions_created_by (created_by),
    CONSTRAINT fk_revisions_review
        FOREIGN KEY (review_id) REFERENCES reviews(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_revisions_created_by
        FOREIGN KEY (created_by) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
