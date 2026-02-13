-- Seed data for default-agent-0000 plugin in verifier database

-- Insert the default-agent-0000 plugin
INSERT INTO plugins (id, title, description, server_endpoint, category) VALUES
(
    'default-agent-0000',
    'Default Agent Plugin',
    'Default agent plugin for testing and development.',
    'http://host.docker.internal:8081',
    'app'
) ON CONFLICT (id) DO NOTHING;

-- Insert API key for the default-agent-0000 plugin
INSERT INTO plugin_apikey (id, plugin_id, apikey, created_at, expires_at, status) VALUES
(
    gen_random_uuid(),
    'default-agent-0000',
    'localhost-agent-apikey',
    now(),
    null,
    1
) ON CONFLICT (apikey) DO NOTHING;

-- Initialize plugin ratings (optional but good for consistency)
INSERT INTO plugin_ratings (plugin_id, avg_rating, total_ratings) VALUES
(
    'default-agent-0000',
    0,
    0
) ON CONFLICT (plugin_id) DO NOTHING;