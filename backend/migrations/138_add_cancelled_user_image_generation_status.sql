ALTER TABLE user_image_generations
    DROP CONSTRAINT IF EXISTS user_image_generations_status_check;

ALTER TABLE user_image_generations
    ADD CONSTRAINT user_image_generations_status_check
        CHECK (status IN ('processing', 'completed', 'failed', 'cancelled'));
