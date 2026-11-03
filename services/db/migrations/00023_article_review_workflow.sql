-- +goose Up
-- Article review is now the pinned article-publish workflow, whose nodes call
-- article.record-review / article.publish / article.reject. The old policy
-- started a second, agent-less moderation pass on the same submit, so it is
-- removed instead of left to duplicate or contradict the flow.
DELETE FROM ai_trigger_policies WHERE policy_id='article-moderation';
-- +goose Down
INSERT INTO ai_trigger_policies (tenant_id,policy_id,version,enabled,event_type,agent,mode,instruction,max_depth)
VALUES ('*','article-moderation',1,true,'article.submitted','article_moderator','async','Review the submitted article.',5)
ON CONFLICT DO NOTHING;
