-- name: ArticleList :many
SELECT id,title,status,revision,review_code,created_by FROM articles
WHERE tenant_id=$1 AND (created_by=$2 OR sqlc.arg(can_review)::boolean)
AND (sqlc.arg(keyword)::text = '' OR strpos(lower(title),lower(sqlc.arg(keyword))) > 0)
AND (sqlc.arg(status_filter)::text = '' OR status=sqlc.arg(status_filter))
ORDER BY
 CASE WHEN sqlc.arg(sort_key)::text='title:asc' THEN title END ASC,
 CASE WHEN sqlc.arg(sort_key)::text='title:desc' THEN title END DESC,
 CASE WHEN sqlc.arg(sort_key)::text='status:asc' THEN status END ASC,
 CASE WHEN sqlc.arg(sort_key)::text='status:desc' THEN status END DESC,
 CASE WHEN sqlc.arg(sort_key)::text='revision:asc' THEN revision END ASC,
 CASE WHEN sqlc.arg(sort_key)::text='revision:desc' THEN revision END DESC,
 CASE WHEN sqlc.arg(sort_key)::text='id:asc' THEN id END ASC,
 id DESC LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);
-- name: ArticleCount :one
SELECT count(*) FROM articles
WHERE tenant_id=$1 AND (created_by=$2 OR sqlc.arg(can_review)::boolean)
AND (sqlc.arg(keyword)::text = '' OR strpos(lower(title),lower(sqlc.arg(keyword))) > 0)
AND (sqlc.arg(status_filter)::text = '' OR status=sqlc.arg(status_filter));
-- name: ArticleGet :one
SELECT id,title,body,status,revision,review_code,created_by FROM articles
WHERE tenant_id=$1 AND id=$2 AND (created_by=$3 OR sqlc.arg(can_review)::boolean);
-- name: ArticleCreate :one
INSERT INTO articles(tenant_id,created_by,title,body) VALUES($1,$2,$3,$4) RETURNING id;
-- name: ArticleUpdate :execrows
UPDATE articles SET title=$4,body=$5,revision=revision+1,status='draft',review_code='',updated_at=now()
WHERE tenant_id=$1 AND id=$2 AND created_by=$3 AND revision=$6 AND status IN ('draft','rejected');
-- Submit enters review and clears any previous verdict, so a retried run never
-- reuses the conclusion of an earlier attempt.
-- name: ArticleSubmit :execrows
UPDATE articles SET status='pending_ai',review_code='',updated_at=now()
WHERE tenant_id=$1 AND id=$2 AND created_by=$3 AND revision=$4 AND status IN ('draft','pending_ai');
-- name: ArticleRecordReview :execrows
UPDATE articles SET review_code=$4,updated_at=now()
WHERE tenant_id=$1 AND id=$2 AND created_by=$3 AND revision=$5 AND status='pending_ai';
-- Publishing is conditional on the verdict already recorded by the review step.
-- name: ArticlePublish :execrows
UPDATE articles SET status='published',updated_at=now()
WHERE tenant_id=$1 AND id=$2 AND created_by=$3 AND revision=$4 AND status='pending_ai' AND review_code='safe';
-- name: ArticleReject :execrows
UPDATE articles SET status='rejected',updated_at=now()
WHERE tenant_id=$1 AND id=$2 AND created_by=$3 AND revision=$4 AND status='pending_ai' AND review_code IN ('blocked','suspected');
-- name: ArticleDecide :execrows
UPDATE articles SET status=$4,reviewed_by=$5,review_code='human_review',updated_at=now() WHERE tenant_id=$1 AND id=$2 AND revision=$3 AND status='pending_review' AND created_by<>$5;
