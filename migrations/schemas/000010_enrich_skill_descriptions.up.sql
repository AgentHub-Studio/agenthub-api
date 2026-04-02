-- Enrich skill descriptions for LLM tool-calling.
-- Only updates skills whose current description is shorter than the enriched version.

UPDATE skill SET description = 'Searches documents in the knowledge base by semantic similarity. Use when the user asks about information that may exist in uploaded documents or company knowledge bases. The ''query'' parameter should be a descriptive sentence or question, not isolated keywords — for example, use ''how to reset a user password'' instead of ''password reset''. Returns relevant text excerpts with similarity scores, sorted by relevance.', updated_at = NOW()
  WHERE slug = 'document-search' AND length(description) < 350;

UPDATE skill SET description = 'Executes SQL queries against configured PostgreSQL datasources. Use when the user needs to query, analyze, or explore structured data. The ''query'' parameter must be valid SQL. Always use SELECT for reads; NEVER execute UPDATE, DELETE, or DROP without explicit user confirmation. Use LIMIT to avoid returning excessive rows. Returns rows as an array of objects with column names as keys.', updated_at = NOW()
  WHERE slug = 'execute-sql' AND length(description) < 350;

UPDATE skill SET description = 'Makes HTTP requests to external APIs and services. Use when the user needs to interact with a REST API, fetch data from a URL, or trigger a webhook. Parameters include ''method'' (GET/POST/PUT/DELETE), ''url'', ''headers'' (object), and ''body'' (JSON). Always include required headers like Content-Type and Authorization. Returns the HTTP status code, response headers, and response body.', updated_at = NOW()
  WHERE slug = 'http-request' AND length(description) < 350;

UPDATE skill SET description = 'Sends an email via the configured email service. Use when the user explicitly asks to send, forward, or reply to an email. ALWAYS confirm recipient, subject, and content with the user before sending. Parameters: ''to'' (email address), ''subject'', ''body'' (plain text or HTML), ''cc'' (optional). Returns a confirmation with the message ID.', updated_at = NOW()
  WHERE slug = 'send-email' AND length(description) < 300;

UPDATE skill SET description = 'Extracts content from a web page given its URL. Use when the user provides a URL and wants to read, summarize, or analyze its content. The ''url'' parameter should be a complete URL including the protocol (https://). Returns the page title, main text content, and metadata. Note: some pages may block automated access.', updated_at = NOW()
  WHERE slug = 'web-scraper' AND length(description) < 300;

UPDATE skill SET description = 'Executes code snippets in a sandboxed environment. Use when the user needs calculations, data transformations, or script execution. The ''code'' parameter should be the complete code to run. The ''language'' parameter specifies the runtime (python, javascript). Returns stdout, stderr, and any generated files or visualizations.', updated_at = NOW()
  WHERE slug = 'code-interpreter' AND length(description) < 300;
