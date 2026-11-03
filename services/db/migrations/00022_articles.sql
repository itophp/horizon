-- +goose Up
CREATE TABLE articles (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 created_by bigint NOT NULL,
 title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
 body text NOT NULL CHECK (length(body) BETWEEN 1 AND 20000),
 status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','pending_ai','pending_review','published','rejected')),
 revision bigint NOT NULL DEFAULT 1,
 review_code text NOT NULL DEFAULT '',
 reviewed_by bigint NOT NULL DEFAULT 0,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id,id)
);
CREATE INDEX articles_owner ON articles(tenant_id,created_by,id DESC);
INSERT INTO ai_trigger_policies (tenant_id,policy_id,version,enabled,event_type,agent,mode,instruction,max_depth)
VALUES ('*','article-moderation',1,true,'article.submitted','article_moderator','async','Review the submitted article.',5);
-- +goose Down
DELETE FROM ai_trigger_policies WHERE policy_id='article-moderation';
DROP TABLE articles;
