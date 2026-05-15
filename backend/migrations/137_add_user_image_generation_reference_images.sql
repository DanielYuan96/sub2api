ALTER TABLE user_image_generations
    ADD COLUMN IF NOT EXISTS reference_images JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN user_image_generations.reference_images IS '用户上传的参考图数组，包含 data URL 或图片 URL 以及文件元信息';
