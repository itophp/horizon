-- name: ArticleList :many
SELECT id,title,status,revision,review_code,created_by FROM articles
WHERE tenant_id=$1 AND (created_by=$2 OR sqlc.arg(can_review)::boolean) ORDER BY id DESC LIMIT 100;
-- name: ArticleGet :one
SELECT id,title,body,status,revision,review_code,created_by FROM articles
WHERE tenant_id=$1 AND id=$2 AND (created_by=$3 OR sqlc.arg(can_review)::boolean);
-- name: ArticleCreate :one
INSERT INTO articles(tenant_id,created_by,title,body) VALUES($1,$2,$3,$4) RETURNING id;
-- name: ArticleUpdate :execrows
UPDATE articles SET title=$4,body=$5,revision=revision+1,status='draft',review_code='',updated_at=now()
WHERE tenant_id=$1 AND id=$2 AND created_by=$3 AND revision=$6 AND status IN ('draft','rejected');
-- name: ArticleSubmit :execrows
UPDATE articles SET status='pending_ai',updated_at=now() WHERE tenant_id=$1 AND id=$2 AND created_by=$3 AND revision=$4 AND status='draft';
-- name: ArticleModerate :execrows
UPDATE articles SET status=$5,review_code=$6,updated_at=now() WHERE tenant_id=$1 AND id=$2 AND created_by=$3 AND revision=$4 AND status='pending_ai';
-- name: ArticleDecide :execrows
UPDATE articles SET status=$4,reviewed_by=$5,review_code='human_review',updated_at=now() WHERE tenant_id=$1 AND id=$2 AND revision=$3 AND status='pending_review' AND created_by<>$5;
