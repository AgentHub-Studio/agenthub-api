-- Skill Evaluation Framework (WP-09)
-- Tables per tenant schema (no tenant_id column — isolation via search_path).

-- skill_eval_suite groups test cases for a single skill.
CREATE TABLE IF NOT EXISTS skill_eval_suite (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_id    UUID         NOT NULL,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_skill_eval_suite_skill_id ON skill_eval_suite(skill_id);

-- skill_eval_case is a single test case within a suite.
-- grader_type: exact_match | contains | regex | llm_judge
CREATE TABLE IF NOT EXISTS skill_eval_case (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    suite_id        UUID         NOT NULL REFERENCES skill_eval_suite(id) ON DELETE CASCADE,
    description     TEXT,
    input_text      TEXT         NOT NULL,
    expected_tool   VARCHAR(255),
    expected_output TEXT,
    grader_type     VARCHAR(50)  NOT NULL DEFAULT 'contains',
    grader_config   JSONB        NOT NULL DEFAULT '{}',
    should_trigger  BOOLEAN      NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_skill_eval_case_suite_id ON skill_eval_case(suite_id);

-- skill_eval_run records one execution of a suite.
-- status: running | passed | failed | error
CREATE TABLE IF NOT EXISTS skill_eval_run (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    suite_id     UUID        NOT NULL REFERENCES skill_eval_suite(id) ON DELETE CASCADE,
    status       VARCHAR(50) NOT NULL DEFAULT 'running',
    total_cases  INT         NOT NULL DEFAULT 0,
    passed_cases INT         NOT NULL DEFAULT 0,
    failed_cases INT         NOT NULL DEFAULT 0,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_skill_eval_run_suite_id ON skill_eval_run(suite_id);

-- skill_eval_case_result stores per-case outcome within a run.
CREATE TABLE IF NOT EXISTS skill_eval_case_result (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id        UUID        NOT NULL REFERENCES skill_eval_run(id) ON DELETE CASCADE,
    case_id       UUID        NOT NULL REFERENCES skill_eval_case(id) ON DELETE CASCADE,
    passed        BOOLEAN     NOT NULL,
    actual_output TEXT,
    score         FLOAT8,
    error_msg     TEXT,
    duration_ms   INT         NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_skill_eval_result_run_id ON skill_eval_case_result(run_id);
