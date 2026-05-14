-- Rollback migration 000093: remove capability-layer hook templates.
-- Removes ONLY the 4 rows inserted by 000093. The hook table itself
-- (created by 000010) is left intact.

DELETE FROM ah_core.hook
WHERE slug IN (
    'capability-posttooluse-cite-web-sources',
    'capability-posttooluse-index-doc-citations',
    'capability-pretooluse-validate-search-query',
    'capability-sessionstart-load-task-context'
);
