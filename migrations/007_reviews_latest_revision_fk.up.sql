UPDATE reviews r
  JOIN (
    SELECT review_id, MAX(id) AS max_rev_id
    FROM review_revisions
    GROUP BY review_id
  ) rr ON r.id = rr.review_id
SET r.latest_revision_id = rr.max_rev_id
WHERE r.latest_revision_id IS NULL;

ALTER TABLE reviews
    ADD CONSTRAINT fk_reviews_latest_revision
        FOREIGN KEY (latest_revision_id)
        REFERENCES review_revisions(id)
        ON DELETE SET NULL;
