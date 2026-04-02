/**
 * AgentHub API — Load Test (k6)
 *
 * Usage:
 *   AUTH_TOKEN=<jwt> BACKEND_URL=http://localhost:8081 k6 run load_test.js
 *
 * Scenarios:
 *   - smoke:    1 VU, 30s — baseline sanity check
 *   - load:     up to 20 VUs, 2min ramp — realistic concurrent users
 *   - stress:   up to 50 VUs, 4min — identify breaking point
 *
 * Thresholds:
 *   - p95 response time < 500ms
 *   - error rate < 1%
 */

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// ── Custom metrics ───────────────────────────────────────────────────────────
const errorRate = new Rate('errors');
const agentCreateDuration = new Trend('agent_create_duration', true);
const pipelineGetDuration = new Trend('pipeline_get_duration', true);

// ── Config ───────────────────────────────────────────────────────────────────
const BASE_URL = __ENV.BACKEND_URL || 'http://localhost:8081';
const TOKEN = __ENV.AUTH_TOKEN || '';

const params = {
  headers: {
    'Content-Type': 'application/json',
    'Authorization': TOKEN ? `Bearer ${TOKEN}` : '',
  },
};

// ── Scenarios ────────────────────────────────────────────────────────────────
export const options = {
  scenarios: {
    smoke: {
      executor: 'constant-vus',
      vus: 1,
      duration: '30s',
      tags: { scenario: 'smoke' },
    },
    load: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '30s', target: 10 },
        { duration: '1m',  target: 20 },
        { duration: '30s', target: 0  },
      ],
      tags: { scenario: 'load' },
      startTime: '35s', // start after smoke
    },
    stress: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '30s', target: 20 },
        { duration: '1m',  target: 50 },
        { duration: '1m',  target: 50 },
        { duration: '30s', target: 0  },
      ],
      tags: { scenario: 'stress' },
      startTime: '3m30s', // start after load
    },
  },
  thresholds: {
    http_req_failed:   ['rate<0.01'],    // <1% errors
    http_req_duration: ['p(95)<500'],    // 95th percentile <500ms
    errors:            ['rate<0.01'],
  },
};

// ── Helper ───────────────────────────────────────────────────────────────────
function assertOK(res, tag) {
  const ok = check(res, {
    [`${tag}: status 2xx`]: (r) => r.status >= 200 && r.status < 300,
  });
  errorRate.add(!ok);
  return ok;
}

// ── Health check (no auth required) ─────────────────────────────────────────
export function healthCheck() {
  const res = http.get(`${BASE_URL}/health`);
  assertOK(res, 'health');
}

// ── Main virtual user flow ───────────────────────────────────────────────────
export default function () {
  group('health', () => {
    const res = http.get(`${BASE_URL}/health`);
    assertOK(res, 'health');
  });

  if (!TOKEN) {
    sleep(1);
    return; // skip authenticated routes if no token
  }

  group('agents', () => {
    // List agents
    const list = http.get(`${BASE_URL}/api/agents?page=0&size=10`, params);
    assertOK(list, 'list agents');

    // Create agent
    const start = Date.now();
    const create = http.post(
      `${BASE_URL}/api/agents`,
      JSON.stringify({
        name: `LoadTest-${__VU}-${__ITER}`,
        description: 'k6 load test agent',
        status: 'DRAFT',
      }),
      params,
    );
    agentCreateDuration.add(Date.now() - start);

    if (!assertOK(create, 'create agent')) {
      sleep(1);
      return;
    }

    const agent = create.json();
    const agentId = agent.id;

    // Get agent
    const get = http.get(`${BASE_URL}/api/agents/${agentId}`, params);
    assertOK(get, 'get agent');

    // Update agent
    const update = http.put(
      `${BASE_URL}/api/agents/${agentId}`,
      JSON.stringify({ name: `LoadTest-${__VU}-${__ITER}-upd`, status: 'DRAFT' }),
      params,
    );
    assertOK(update, 'update agent');

    // Delete agent
    const del = http.del(`${BASE_URL}/api/agents/${agentId}`, null, params);
    check(del, { 'delete agent: 204': (r) => r.status === 204 });
  });

  group('pipelines', () => {
    // Create pipeline
    const create = http.post(
      `${BASE_URL}/api/pipelines`,
      JSON.stringify({ name: `Pipeline-${__VU}-${__ITER}`, description: 'load test' }),
      params,
    );
    if (!assertOK(create, 'create pipeline')) {
      sleep(1);
      return;
    }

    const pipeline = create.json();
    const pipelineId = pipeline.id;

    // Get graph
    const graphStart = Date.now();
    const graph = http.get(`${BASE_URL}/api/pipelines/${pipelineId}/graph`, params);
    pipelineGetDuration.add(Date.now() - graphStart);
    assertOK(graph, 'get graph');

    // List pipelines
    const list = http.get(`${BASE_URL}/api/pipelines?page=0&size=10`, params);
    assertOK(list, 'list pipelines');

    // Cleanup
    http.del(`${BASE_URL}/api/pipelines/${pipelineId}`, null, params);
  });

  group('knowledge-bases', () => {
    const list = http.get(`${BASE_URL}/api/knowledge-bases?page=0&size=10`, params);
    assertOK(list, 'list knowledge-bases');
  });

  group('chat', () => {
    const sessions = http.get(`${BASE_URL}/api/chat/sessions?size=10`, params);
    assertOK(sessions, 'list chat sessions');
  });

  sleep(Math.random() * 2 + 0.5); // 0.5–2.5s think time
}
