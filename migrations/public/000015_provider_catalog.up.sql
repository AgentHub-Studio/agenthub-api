-- Curated catalog of integration providers that tenants can instantiate
-- in one click (GitHub MCP, PostgreSQL, etc.). Lives in public schema
-- because the catalog itself is global — not per-tenant.

CREATE TABLE provider_template (
    slug          VARCHAR(100) PRIMARY KEY,
    name          VARCHAR(255) NOT NULL,
    description   TEXT,
    icon          VARCHAR(500),
    category      VARCHAR(100),
    kind          VARCHAR(50) NOT NULL,
    template_json JSONB NOT NULL,
    is_builtin    BOOLEAN NOT NULL DEFAULT false,
    enabled       BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT provider_template_kind_chk CHECK (kind IN ('http', 'database', 'mcp'))
);

CREATE INDEX idx_provider_template_kind ON provider_template(kind);
CREATE INDEX idx_provider_template_category ON provider_template(category);
CREATE INDEX idx_provider_template_enabled ON provider_template(enabled);

-- Seed builtin providers. Keeping the template_json minimal for each;
-- integration.Service fills in tenant-specific credentials at instantiate time.

INSERT INTO provider_template (slug, name, description, icon, category, kind, is_builtin, template_json) VALUES
-- MCP providers (OAuth-backed servers)
('github-mcp', 'GitHub', 'Consulte repositórios, issues e PRs via MCP.', 'bi-github', 'Desenvolvimento', 'mcp', true,
 '{"serverUrl":"https://api.githubcopilot.com/mcp/","authType":"oauth","oauthProvider":"github"}'::jsonb),
('google-drive-mcp', 'Google Drive', 'Leia e busque arquivos do seu Drive via MCP.', 'bi-google', 'Produtividade', 'mcp', true,
 '{"serverUrl":"https://mcp.google.com/drive","authType":"oauth","oauthProvider":"google"}'::jsonb),

-- HTTP providers (REST APIs with OAuth or token auth)
('slack-api', 'Slack', 'Envie e leia mensagens dos seus canais.', 'bi-slack', 'Comunicação', 'http', true,
 '{"baseUrl":"https://slack.com/api","authType":"oauth","oauthProvider":"slack"}'::jsonb),
('gmail-api', 'Gmail', 'Envie emails e busque mensagens.', 'bi-envelope', 'Comunicação', 'http', true,
 '{"baseUrl":"https://gmail.googleapis.com/gmail/v1","authType":"oauth","oauthProvider":"google"}'::jsonb),
('jira-api', 'Jira', 'Busque e atualize issues do seu projeto.', 'bi-kanban', 'Desenvolvimento', 'http', true,
 '{"baseUrl":"https://{workspace}.atlassian.net","authType":"basic","placeholders":{"workspace":"seu-dominio"}}'::jsonb),
('hubspot-api', 'HubSpot', 'Consulte contatos, empresas e negócios.', 'bi-briefcase', 'CRM', 'http', true,
 '{"baseUrl":"https://api.hubapi.com","authType":"bearer"}'::jsonb),
('generic-http', 'API genérica (REST)', 'Conecte qualquer API REST informando a URL base.', 'bi-globe', 'Outros', 'http', true,
 '{"baseUrl":"","authType":"none","placeholders":{"baseUrl":"https://api.seu-sistema.com"}}'::jsonb),

-- Database providers (require VPN or public accessibility)
('postgresql', 'PostgreSQL', 'Consulte seu banco PostgreSQL.', 'bi-database', 'Banco de Dados', 'database', true,
 '{"type":"POSTGRESQL","port":5432,"placeholders":{"host":"seu-host.example.com","database":"nome-do-banco","dbUser":"usuario"}}'::jsonb),
('mysql', 'MySQL', 'Consulte seu banco MySQL ou MariaDB.', 'bi-database', 'Banco de Dados', 'database', true,
 '{"type":"MYSQL","port":3306,"placeholders":{"host":"seu-host.example.com","database":"nome-do-banco","dbUser":"usuario"}}'::jsonb),
('sqlserver', 'SQL Server', 'Consulte seu banco Microsoft SQL Server.', 'bi-database', 'Banco de Dados', 'database', true,
 '{"type":"SQL_SERVER","port":1433,"placeholders":{"host":"seu-host.example.com","database":"nome-do-banco","dbUser":"usuario"}}'::jsonb);
