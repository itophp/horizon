# 文章应用

继承 [应用规则](../AGENTS.md)。

文章审核遵循 [安全约定](../../docs/security.md)的文章评审规则：正文不落调用/AI 审计；评审结论由 article.record-review 落库并归一化为 decision（只接受 OriginWorkflow），发布声明 high + confirm，必须人工确认（多人部署可切 four-eyes），article.reject 只接受 OriginWorkflow|OriginApproval，且都要求 review_code 与状态、版本同时匹配；人工审核禁止自动化与作者自审；修改正文或版本不得沿用旧审核。条件分叉用 Step 的 when 声明，禁止用"两个都跑、内部判空"的方式伪造分支。
