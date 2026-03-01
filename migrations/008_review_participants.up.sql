CREATE TABLE review_participants (
    id             BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    review_id      BIGINT UNSIGNED  NOT NULL,
    user_id        BIGINT UNSIGNED  NOT NULL,
    role           ENUM('author','reviewer','blocking_reviewer','subscriber')
                                    NOT NULL,
    state          ENUM('pending','commented','changes_requested','accepted','watching')
                                    NOT NULL DEFAULT 'pending',
    added_by       BIGINT UNSIGNED  NOT NULL,
    added_reason   ENUM('manual','path_owner_rule','mention','fork_suggestion')
                                    NOT NULL,
    created_at     DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_participants_review_user (review_id, user_id),
    INDEX idx_participants_user (user_id),
    CONSTRAINT fk_participants_review
        FOREIGN KEY (review_id) REFERENCES reviews(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_participants_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_participants_added_by
        FOREIGN KEY (added_by) REFERENCES users(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
