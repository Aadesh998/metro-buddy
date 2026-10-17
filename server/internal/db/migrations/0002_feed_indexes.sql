CREATE INDEX IF NOT EXISTS idx_posts_bowl_hot ON posts (bowl_id, hot_rank DESC, id DESC) WHERE deleted_at IS NULL AND status = 'visible';
CREATE INDEX IF NOT EXISTS idx_posts_bowl_new ON posts (bowl_id, created_at DESC, id DESC) WHERE deleted_at IS NULL AND status = 'visible';
CREATE INDEX IF NOT EXISTS idx_posts_bowl_top ON posts (bowl_id, score DESC, id DESC) WHERE deleted_at IS NULL AND status = 'visible';
CREATE INDEX IF NOT EXISTS idx_comments_post_id ON comments (post_id, id);
CREATE INDEX IF NOT EXISTS idx_stations_search_trgm ON stations USING gin (search_text gin_trgm_ops);
