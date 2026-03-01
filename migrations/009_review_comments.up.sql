CREATE TABLE review_comments (
    id                    BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    review_id             BIGINT UNSIGNED  NOT NULL,
    revision_id           BIGINT UNSIGNED  NOT NULL,
    author_id             BIGINT UNSIGNED  NOT NULL,
    parent_comment_id     BIGINT UNSIGNED  NULL,
    publish_batch_id      CHAR(36)         NULL COMMENT 'UUID grouping a publish batch',
    file_path             VARCHAR(1024)    NULL,
    side                  ENUM('left','right') NULL,
    start_line            INT UNSIGNED     NULL,
    end_line              INT UNSIGNED     NULL,
    body                  TEXT             NOT NULL,
    visibility            ENUM('draft','published')
                                           NOT NULL DEFAULT 'draft',
    is_resolved           BOOLEAN          NOT NULL DEFAULT FALSE,
    context_fingerprint   CHAR(64)         NULL COMMENT 'SHA-256 hex of surrounding context; set at publish',
    anchor_method         ENUM('original','fingerprint','offset','orphaned')
                                           NOT NULL DEFAULT 'original',
    orphaned_at           DATETIME         NULL,
    created_at            DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at          DATETIME         NULL,
    resolved_at           DATETIME         NULL,
    PRIMARY KEY (id),
    INDEX idx_comments_review_vis (review_id, visibility, created_at),
    INDEX idx_comments_review_file (review_id, revision_id, file_path(255)),
    INDEX idx_comments_author_draft (author_id, visibility),
    INDEX idx_comments_parent (parent_comment_id),
    CONSTRAINT fk_comments_review
        FOREIGN KEY (review_id) REFERENCES reviews(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_comments_revision
        FOREIGN KEY (revision_id) REFERENCES review_revisions(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_comments_author
        FOREIGN KEY (author_id) REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_comments_parent
        FOREIGN KEY (parent_comment_id) REFERENCES review_comments(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
