/**
 * AgentHub API — Load Test (k6)
 *
 * Usage:
 *   AUTH_TOKEN=<jwt> BACKEND_URL=http://localhost:8081 k6 run load_test.js
 *   AUTH_URL=http://localhost:8080/realms/test/protocol/openid-connect/token \
 *     AUTH_USERNAME=admin@test.dev.local AUTH_PASSWORD=<password> \
 *     BACKEND_URL=http://localhost:8081 k6 run load_test.js
 *   PERF_PROFILE=smoke AUTH_TOKEN=<jwt> BACKEND_URL=http://localhost:8081 k6 run load_test.js
 *   PERF_PROFILE=load LOAD_MAX_VUS=5 AUTH_TOKEN=<jwt> BACKEND_URL=http://localhost:8081 k6 run load_test.js
 *   PERF_PROFILE=stress STRESS_MAX_VUS=10 AUTH_TOKEN=<jwt> BACKEND_URL=http://localhost:8081 k6 run load_test.js
 *   PERF_PROFILE=smoke INCLUDE_BROAD_READS=true AUTH_URL=... AUTH_USERNAME=... AUTH_PASSWORD=... \
 *     BACKEND_URL=http://localhost:8081 k6 run load_test.js
 *   PERF_PROFILE=a2a AUTH_BASE_URL=http://localhost:8080 \
 *     AUTH_PASSWORD=<password> BACKEND_URL=http://localhost:8081 k6 run load_test.js
 *
 * Scenarios:
 *   - smoke:    1 VU, 30s — baseline sanity check
 *   - load:     up to 20 VUs, 2min ramp — realistic concurrent users
 *   - stress:   up to 50 VUs, 4min — identify breaking point
 *   - a2a:      sustained A2A invokes across tenant pairs, below rate limit
 *
 * Thresholds:
 *   - p95 response time < 500ms (1.5s for the A2A profile)
 *   - error rate < 1%
 */

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// ── Custom metrics ───────────────────────────────────────────────────────────
const errorRate = new Rate('errors');
const agentCreateDuration = new Trend('agent_create_duration', true);
const pipelineGetDuration = new Trend('pipeline_get_duration', true);
const broadReadDuration = new Trend('broad_read_duration', true);
const a2aInvokeDuration = new Trend('a2a_invoke_duration', true);
const a2aInvokeTTFB = new Trend('a2a_invoke_ttfb', true);
const a2aStreamReceiveDuration = new Trend('a2a_stream_receive_duration', true);
const a2aGrantCheckDuration = new Trend('a2a_grant_check_duration', true);
const a2aRateLimitDuration = new Trend('a2a_rate_limit_duration', true);
const a2aAgentLoadDuration = new Trend('a2a_agent_load_duration', true);
const a2aSessionCreateDuration = new Trend('a2a_session_create_duration', true);
const a2aRunPrepareDuration = new Trend('a2a_run_prepare_duration', true);
const a2aPreparationDuration = new Trend('a2a_preparation_duration', true);
const a2aRateLimited = new Rate('a2a_rate_limited');

// ── Config ───────────────────────────────────────────────────────────────────
const BASE_URL = __ENV.BACKEND_URL || 'http://localhost:8081';
const STATIC_TOKEN = __ENV.AUTH_TOKEN || '';
const AUTH_URL = __ENV.AUTH_URL || '';
const AUTH_BASE_URL = __ENV.AUTH_BASE_URL || authBaseURLFrom(AUTH_URL);
const AUTH_CLIENT_ID = __ENV.AUTH_CLIENT_ID || 'agenthub-frontend';
const AUTH_USERNAME = __ENV.AUTH_USERNAME || '';
const AUTH_PASSWORD = __ENV.AUTH_PASSWORD || '';
const AUTH_REFRESH_SKEW_SECONDS = Number(__ENV.AUTH_REFRESH_SKEW_SECONDS || 30);
const PROFILE = __ENV.PERF_PROFILE || 'full';
const DEFAULT_RESPONSE_P95_LIMIT_MS = 500;
const A2A_RESPONSE_P95_LIMIT_MS = 1500;
const SMOKE_DURATION = __ENV.SMOKE_DURATION || '30s';
const LOAD_MAX_VUS = Number(__ENV.LOAD_MAX_VUS || 20);
const LOAD_RAMP_DURATION = __ENV.LOAD_RAMP_DURATION || '30s';
const LOAD_HOLD_DURATION = __ENV.LOAD_HOLD_DURATION || '1m';
const LOAD_RAMP_DOWN_DURATION = __ENV.LOAD_RAMP_DOWN_DURATION || '30s';
const STRESS_MAX_VUS = Number(__ENV.STRESS_MAX_VUS || 50);
const STRESS_RAMP_DURATION = __ENV.STRESS_RAMP_DURATION || '30s';
const STRESS_HOLD_DURATION = __ENV.STRESS_HOLD_DURATION || '1m';
const STRESS_PEAK_HOLD_DURATION = __ENV.STRESS_PEAK_HOLD_DURATION || '1m';
const STRESS_RAMP_DOWN_DURATION = __ENV.STRESS_RAMP_DOWN_DURATION || '30s';
const INCLUDE_DEPRECATED_PIPELINE_WRITES = __ENV.INCLUDE_DEPRECATED_PIPELINE_WRITES === 'true';
const INCLUDE_BROAD_READS = __ENV.INCLUDE_BROAD_READS === 'true';
const RUN_ID = __ENV.RUN_ID || String(Date.now());
const A2A_VUS = Number(__ENV.A2A_VUS || 3);
const A2A_DURATION = __ENV.A2A_DURATION || '5m';
const A2A_SLEEP_SECONDS = Number(__ENV.A2A_SLEEP_SECONDS || 2);
const A2A_PAIRS = parseA2APairs(__ENV.A2A_PAIRS || 'tenant-a:tenant-b,tenant-b:tenant-c,tenant-c:tenant-a');

let vuToken = STATIC_TOKEN;
let vuTokenExpiresAt = STATIC_TOKEN && !canRefreshToken() ? Number.MAX_SAFE_INTEGER : 0;
let tenantTokens = {};

// ── Scenarios ────────────────────────────────────────────────────────────────
const smokeScenario = {
  smoke: {
    executor: 'constant-vus',
    vus: 1,
    duration: SMOKE_DURATION,
    tags: { scenario: 'smoke' },
  },
};

const loadScenario = {
  load: {
    executor: 'ramping-vus',
    startVUs: 1,
    stages: [
      { duration: LOAD_RAMP_DURATION, target: Math.max(1, Math.floor(LOAD_MAX_VUS / 2)) },
      { duration: LOAD_HOLD_DURATION, target: LOAD_MAX_VUS },
      { duration: LOAD_RAMP_DOWN_DURATION, target: 0 },
    ],
    tags: { scenario: 'load' },
  },
};

const stressScenario = {
  stress: {
    executor: 'ramping-vus',
    startVUs: 1,
    stages: [
      { duration: STRESS_RAMP_DURATION, target: Math.max(1, Math.floor(STRESS_MAX_VUS * 0.4)) },
      { duration: STRESS_HOLD_DURATION, target: STRESS_MAX_VUS },
      { duration: STRESS_PEAK_HOLD_DURATION, target: STRESS_MAX_VUS },
      { duration: STRESS_RAMP_DOWN_DURATION, target: 0 },
    ],
    tags: { scenario: 'stress' },
  },
};

const a2aScenario = {
  a2a: {
    executor: 'constant-vus',
    vus: A2A_VUS,
    duration: A2A_DURATION,
    tags: { scenario: 'a2a' },
  },
};

const fullScenarios = Object.assign({}, smokeScenario, {
  load: Object.assign({}, loadScenario.load, {
    startTime: '35s', // start after smoke
  }),
  stress: Object.assign({}, stressScenario.stress, {
    startTime: '3m30s', // start after load
  }),
});

function selectedScenarios() {
  if (PROFILE === 'smoke') return smokeScenario;
  if (PROFILE === 'load') return loadScenario;
  if (PROFILE === 'stress') return stressScenario;
  if (PROFILE === 'a2a') return a2aScenario;
  return fullScenarios;
}

function selectedThresholds() {
  const responseP95Limit = PROFILE === 'a2a'
    ? A2A_RESPONSE_P95_LIMIT_MS
    : DEFAULT_RESPONSE_P95_LIMIT_MS;
  const thresholds = {
    http_req_failed:   ['rate<0.01'],    // <1% errors
    http_req_duration: [`p(95)<${responseP95Limit}`],
    errors:            ['rate<0.01'],
  };
  if (PROFILE === 'a2a') {
    thresholds.a2a_rate_limited = ['rate==0'];
    thresholds.a2a_invoke_duration = [`p(95)<${A2A_RESPONSE_P95_LIMIT_MS}`];
  }
  return thresholds;
}

export const options = {
  scenarios: selectedScenarios(),
  thresholds: selectedThresholds(),
};

// ── Helper ───────────────────────────────────────────────────────────────────
function assertOK(res, tag) {
  const ok = check(res, {
    [`${tag}: status 2xx`]: (r) => r.status >= 200 && r.status < 300,
  });
  errorRate.add(!ok);
  return ok;
}

function canRefreshToken() {
  return AUTH_URL && AUTH_USERNAME && AUTH_PASSWORD;
}

function authBaseURLFrom(authURL) {
  const marker = '/realms/';
  const index = authURL.indexOf(marker);
  if (index === -1) {
    return '';
  }
  return authURL.slice(0, index);
}

function refreshToken() {
  if (!canRefreshToken()) {
    return vuToken;
  }

  const res = http.post(
    AUTH_URL,
    {
      grant_type: 'password',
      client_id: AUTH_CLIENT_ID,
      username: AUTH_USERNAME,
      password: AUTH_PASSWORD,
    },
    {
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      tags: { name: 'auth token' },
    },
  );

  if (!assertOK(res, 'auth token')) {
    return '';
  }

  const body = res.json();
  vuToken = body.access_token || '';
  const expiresIn = Number(body.expires_in || 300);
  vuTokenExpiresAt = Math.floor(Date.now() / 1000) + expiresIn;
  return vuToken;
}

function currentToken() {
  if (!canRefreshToken()) {
    return vuToken;
  }
  const now = Math.floor(Date.now() / 1000);
  if (!vuToken || now >= (vuTokenExpiresAt - AUTH_REFRESH_SKEW_SECONDS)) {
    return refreshToken();
  }
  return vuToken;
}

function authenticatedParams() {
  const token = currentToken();
  return {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': token ? `Bearer ${token}` : '',
    },
  };
}

function parseA2APairs(value) {
  const pairs = [];
  value.split(',').forEach((entry) => {
    const parts = entry.split(':');
    if (parts.length !== 2) {
      return;
    }
    const sourceTenant = parts[0].trim();
    const targetTenant = parts[1].trim();
    if (sourceTenant && targetTenant && sourceTenant !== targetTenant) {
      pairs.push({ sourceTenant, targetTenant });
    }
  });
  if (pairs.length === 0) {
    throw new Error('A2A_PAIRS must contain at least one source:target pair');
  }
  return pairs;
}

function authURLForTenant(tenant) {
  if (!AUTH_BASE_URL) {
    throw new Error('AUTH_BASE_URL or AUTH_URL is required for PERF_PROFILE=a2a');
  }
  return `${AUTH_BASE_URL}/realms/${tenant}/protocol/openid-connect/token`;
}

function usernameForTenant(tenant) {
  return `admin@${tenant}.dev.local`;
}

function refreshTenantToken(tenant) {
  if (!AUTH_PASSWORD) {
    return '';
  }

  const res = http.post(
    authURLForTenant(tenant),
    {
      grant_type: 'password',
      client_id: AUTH_CLIENT_ID,
      username: usernameForTenant(tenant),
      password: AUTH_PASSWORD,
    },
    {
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      tags: { name: 'auth token tenant' },
    },
  );

  if (!assertOK(res, `auth token ${tenant}`)) {
    return '';
  }

  const body = res.json();
  const expiresIn = Number(body.expires_in || 300);
  tenantTokens[tenant] = {
    token: body.access_token || '',
    expiresAt: Math.floor(Date.now() / 1000) + expiresIn,
  };
  return tenantTokens[tenant].token;
}

function tokenForTenant(tenant) {
  const now = Math.floor(Date.now() / 1000);
  const cached = tenantTokens[tenant];
  if (!cached || now >= (cached.expiresAt - AUTH_REFRESH_SKEW_SECONDS)) {
    return refreshTenantToken(tenant);
  }
  return cached.token;
}

function tenantParams(tenant) {
  const token = tokenForTenant(tenant);
  return {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': token ? `Bearer ${token}` : '',
    },
  };
}

function namedParams(params, name) {
  return Object.assign({}, params, {
    tags: Object.assign({}, params.tags || {}, { name }),
  });
}

function a2aStartedPreparation(body) {
  const frames = String(body || '').split('\n\n');
  for (let i = 0; i < frames.length; i += 1) {
    const lines = frames[i].split('\n');
    let started = false;
    let data = '';
    for (let j = 0; j < lines.length; j += 1) {
      if (lines[j] === 'event: a2a_started') {
        started = true;
      } else if (lines[j].indexOf('data: ') === 0) {
        data += lines[j].slice(6);
      }
    }
    if (!started || !data) {
      continue;
    }
    try {
      const event = JSON.parse(data);
      return event.preparation || null;
    } catch (_) {
      return null;
    }
  }
  return null;
}

function addA2APreparationMetrics(body) {
  const preparation = a2aStartedPreparation(body);
  if (!preparation) {
    return;
  }
  const metrics = [
    ['grantCheckMs', a2aGrantCheckDuration],
    ['rateLimitMs', a2aRateLimitDuration],
    ['agentLoadMs', a2aAgentLoadDuration],
    ['sessionCreateMs', a2aSessionCreateDuration],
    ['runPrepareMs', a2aRunPrepareDuration],
    ['totalMs', a2aPreparationDuration],
  ];
  for (let i = 0; i < metrics.length; i += 1) {
    const value = preparation[metrics[i][0]];
    if (typeof value === 'number' && value >= 0) {
      metrics[i][1].add(value);
    }
  }
}

const BROAD_READ_ENDPOINTS = [
  ['auth capabilities', '/api/auth/capabilities'],
  ['settings list', '/api/settings'],
  ['settings providers', '/api/settings/providers'],
  ['llm preset list', '/api/llm-config-presets?page=0&size=10'],
  ['skill list', '/api/skills?page=0&size=10'],
  ['tool list', '/api/tools?page=0&size=10'],
  ['tool label list', '/api/tools/labels'],
  ['integration list', '/api/integrations?page=0&size=10'],
  ['datasource list', '/api/datasources/?page=0&size=10'],
  ['vpn resource list', '/api/vpn-resources/?page=0&size=10'],
  ['mcp config list', '/api/mcp-server-configs?page=0&size=10'],
  ['webhook list', '/api/webhooks?page=0&size=10'],
  ['audit log list', '/api/audit-logs?page=0&size=10'],
  ['package list', '/api/packages?page=0&size=10'],
  ['package mine list', '/api/packages/mine?page=0&size=10'],
  ['marketplace listing list', '/api/marketplace/listings?page=0&size=10'],
  ['marketplace installation list', '/api/marketplace/installations?page=0&size=10'],
];

function broadReadMix(params) {
  if (!INCLUDE_BROAD_READS) {
    return;
  }

  group('broad-read-mix', () => {
    BROAD_READ_ENDPOINTS.forEach(([name, path]) => {
      const start = Date.now();
      const res = http.get(`${BASE_URL}${path}`, namedParams(params, name));
      broadReadDuration.add(Date.now() - start, { endpoint: name });
      assertOK(res, name);
    });
  });
}

export function setup() {
  if (PROFILE !== 'a2a') {
    return {};
  }

  const preparedPairs = [];
  A2A_PAIRS.forEach((pair, index) => {
    const targetParams = tenantParams(pair.targetTenant);
    if (!targetParams.headers.Authorization) {
      throw new Error(`missing target token for ${pair.targetTenant}`);
    }

    const agent = http.post(
      `${BASE_URL}/api/agents`,
      JSON.stringify({
        name: `A2ALoad-${RUN_ID}-${index}`,
        description: 'k6 A2A load test agent',
        systemPrompt: 'Respond briefly to the user message.',
        modelConfig: { provider: 'ollama', model: 'agenthub-e2e-fake', temperature: 0 },
      }),
      namedParams(targetParams, 'a2a setup create agent'),
    );
    if (!check(agent, { 'a2a setup create agent: 201': (r) => r.status === 201 })) {
      throw new Error(`failed to create A2A load agent for ${pair.targetTenant}: ${agent.status}`);
    }

    const agentID = agent.json().id;
    const publish = http.post(
      `${BASE_URL}/api/agents/${agentID}/publish`,
      null,
      namedParams(targetParams, 'a2a setup publish agent'),
    );
    if (!check(publish, {
      'a2a setup publish agent: 200 or 204': (r) => r.status === 200 || r.status === 204,
    })) {
      throw new Error(`failed to publish A2A load agent for ${pair.targetTenant}: ${publish.status}`);
    }

    const grant = http.post(
      `${BASE_URL}/api/a2a/grants`,
      JSON.stringify({
        subjectTenant: pair.sourceTenant,
        agentId: agentID,
        actions: ['invoke'],
      }),
      namedParams(targetParams, 'a2a setup create grant'),
    );
    if (!check(grant, { 'a2a setup create grant: 201': (r) => r.status === 201 })) {
      throw new Error(`failed to create A2A grant ${pair.sourceTenant}->${pair.targetTenant}: ${grant.status}`);
    }

    preparedPairs.push(Object.assign({}, pair, { agentID }));
  });

  return { a2aPairs: preparedPairs };
}

// ── Health check (no auth required) ─────────────────────────────────────────
export function healthCheck() {
  const res = http.get(`${BASE_URL}/health`, namedParams({}, 'health'));
  assertOK(res, 'health');
}

function a2aFlow(data) {
  const pairs = data && data.a2aPairs ? data.a2aPairs : [];
  if (pairs.length === 0) {
    errorRate.add(true);
    sleep(1);
    return;
  }

  const pair = pairs[(__VU + __ITER) % pairs.length];
  const params = tenantParams(pair.sourceTenant);
  if (!params.headers.Authorization) {
    errorRate.add(true);
    sleep(1);
    return;
  }

  const start = Date.now();
  const invoke = http.post(
    `${BASE_URL}/api/a2a/invoke`,
    JSON.stringify({
      targetTenant: pair.targetTenant,
      agentId: pair.agentID,
      input: `a2a-load-${RUN_ID}-${__VU}-${__ITER}`,
    }),
    namedParams(params, 'a2a invoke'),
  );
  a2aInvokeDuration.add(Date.now() - start);
  a2aInvokeTTFB.add(invoke.timings.waiting);
  a2aStreamReceiveDuration.add(invoke.timings.receiving);
  addA2APreparationMetrics(invoke.body);
  a2aRateLimited.add(invoke.status === 429);

  const ok = check(invoke, {
    'a2a invoke: status 200': (r) => r.status === 200,
    'a2a invoke: SSE content type': (r) => {
      const contentType = r.headers['Content-Type'] || r.headers['content-type'] || '';
      return contentType.includes('text/event-stream');
    },
    'a2a invoke: started event': (r) => r.body.includes('event: a2a_started'),
    'a2a invoke: runner event': (r) => /event: (text_delta|run_complete)/.test(r.body),
  });
  errorRate.add(!ok);
  sleep(A2A_SLEEP_SECONDS);
}

// ── Main virtual user flow ───────────────────────────────────────────────────
export default function (data) {
  if (PROFILE === 'a2a') {
    a2aFlow(data);
    return;
  }

  group('health', () => {
    const res = http.get(`${BASE_URL}/health`, namedParams({}, 'health'));
    assertOK(res, 'health');
  });

  const params = authenticatedParams();
  if (!params.headers.Authorization) {
    sleep(1);
    return; // skip authenticated routes if no token
  }

  group('agents', () => {
    // List agents
    const list = http.get(`${BASE_URL}/api/agents?page=0&size=10`, namedParams(params, 'list agents'));
    assertOK(list, 'list agents');

    // Create agent
    const start = Date.now();
    const create = http.post(
      `${BASE_URL}/api/agents`,
      JSON.stringify({
        name: `LoadTest-${RUN_ID}-${__VU}-${__ITER}`,
        description: 'k6 load test agent',
        status: 'DRAFT',
      }),
      namedParams(params, 'create agent'),
    );
    agentCreateDuration.add(Date.now() - start);

    if (!assertOK(create, 'create agent')) {
      sleep(1);
      return;
    }

    const agent = create.json();
    const agentId = agent.id;

    // Get agent
    const get = http.get(`${BASE_URL}/api/agents/${agentId}`, namedParams(params, 'get agent'));
    assertOK(get, 'get agent');

    // Update agent
    const update = http.put(
      `${BASE_URL}/api/agents/${agentId}`,
      JSON.stringify({ name: `LoadTest-${RUN_ID}-${__VU}-${__ITER}-upd`, status: 'DRAFT' }),
      namedParams(params, 'update agent'),
    );
    assertOK(update, 'update agent');

    // Delete agent
    const del = http.del(`${BASE_URL}/api/agents/${agentId}`, null, namedParams(params, 'delete agent'));
    check(del, { 'delete agent: 204': (r) => r.status === 204 });
  });

  group('pipelines', () => {
    // Pipelines are deprecated and read-only by default; agentic execution is the active path.
    const list = http.get(`${BASE_URL}/api/pipelines?page=0&size=10`, namedParams(params, 'list pipelines'));
    assertOK(list, 'list pipelines');

    if (!INCLUDE_DEPRECATED_PIPELINE_WRITES) {
      return;
    }

    // Create pipeline
    const create = http.post(
      `${BASE_URL}/api/pipelines`,
      JSON.stringify({ name: `Pipeline-${RUN_ID}-${__VU}-${__ITER}`, description: 'load test' }),
      namedParams(params, 'create pipeline'),
    );
    if (!assertOK(create, 'create pipeline')) {
      sleep(1);
      return;
    }

    const pipeline = create.json();
    const pipelineId = pipeline.id;

    // Get graph
    const graphStart = Date.now();
    const graph = http.get(`${BASE_URL}/api/pipelines/${pipelineId}/graph`, namedParams(params, 'get pipeline graph'));
    pipelineGetDuration.add(Date.now() - graphStart);
    assertOK(graph, 'get graph');

    // Cleanup
    http.del(`${BASE_URL}/api/pipelines/${pipelineId}`, null, namedParams(params, 'delete pipeline'));
  });

  group('knowledge-bases', () => {
    const list = http.get(`${BASE_URL}/api/knowledge-bases?page=0&size=10`, namedParams(params, 'list knowledge-bases'));
    assertOK(list, 'list knowledge-bases');
  });

  group('chat', () => {
    const sessions = http.get(`${BASE_URL}/api/chat/sessions?size=10`, namedParams(params, 'list chat sessions'));
    assertOK(sessions, 'list chat sessions');
  });

  broadReadMix(params);

  sleep(Math.random() * 2 + 0.5); // 0.5–2.5s think time
}
