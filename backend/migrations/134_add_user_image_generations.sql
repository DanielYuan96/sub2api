CREATE TABLE IF NOT EXISTS user_image_generations (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id      BIGINT       NULL REFERENCES api_keys(id) ON DELETE SET NULL,
    api_key_name    VARCHAR(100) NOT NULL DEFAULT '',
    model           VARCHAR(200) NOT NULL,
    size            VARCHAR(50)  NOT NULL DEFAULT '',
    prompt          TEXT         NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'processing',
    images          JSONB        NOT NULL DEFAULT '[]'::jsonb,
    error_message   TEXT         NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ  NULL,
    CONSTRAINT user_image_generations_status_check
        CHECK (status IN ('processing', 'completed', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_user_image_generations_user_created
    ON user_image_generations (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_image_generations_api_key
    ON user_image_generations (api_key_id);

COMMENT ON TABLE user_image_generations IS '用户前端生图页面的提示词、状态和结果历史';
COMMENT ON COLUMN user_image_generations.images IS '生成图片数组，包含 url 或 b64 data URL 以及 revised_prompt';
