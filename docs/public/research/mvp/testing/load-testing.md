# Load Testing Guide

## Overview

Load testing validates system performance under expected and peak traffic conditions. Tests measure latency, throughput, error rates, and resource utilization.

## Framework

- **k6** for load testing scripts
- **Grafana** for visualization (optional)
- **InfluxDB** for metrics storage (optional)

## Performance Targets

| Metric | Target | Measurement |
|--------|--------|-------------|
| API latency (p95) | < 150ms | End-to-end response time |
| Dashboard load | < 1.5s | First contentful paint |
| Concurrent merchants | 100+ | Simultaneous active sessions |
| Error rate | < 0.1% | 5xx responses under load |
| Throughput | 1000+ RPS | Successful requests per second |

## k6 Scripts

### API Latency Test

```javascript
// load/api-latency.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 50 },   // Ramp up
    { duration: '1m', target: 50 },    // Steady state
    { duration: '30s', target: 100 },  // Peak load
    { duration: '1m', target: 100 },   // Sustained peak
    { duration: '30s', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<150', 'p(99)<300'],
    http_req_failed: ['rate<0.01'],
    http_reqs: ['rate>500'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const AUTH_TOKEN = __ENV.AUTH_TOKEN || '';

export default function () {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${AUTH_TOKEN}`,
  };

  const res = http.get(`${BASE_URL}/api/v1/subscriptions`, { headers });

  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 150ms': (r) => r.timings.duration < 150,
    'has data': (r) => JSON.parse(r.body).data !== undefined,
  });

  sleep(1);
}
```

### Dashboard Load Test

```javascript
// load/dashboard-load.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '1m', target: 100 },
    { duration: '2m', target: 100 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<1500'],
    http_req_failed: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  // Simulate dashboard page load with multiple API calls
  const batchRes = http.batch([
    ['GET', `${BASE_URL}/api/v1/dashboard/stats`, null, { tags: { name: 'stats' } }],
    ['GET', `${BASE_URL}/api/v1/transactions?limit=20`, null, { tags: { name: 'transactions' } }],
    ['GET', `${BASE_URL}/api/v1/subscriptions?limit=10`, null, { tags: { name: 'subscriptions' } }],
  ]);

  batchRes.forEach((res) => {
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 1500ms': (r) => r.timings.duration < 1500,
    });
  });

  sleep(2);
}
```

### Concurrent Merchant Test

```javascript
// load/concurrent-merchants.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '2m', target: 100 },   // Ramp to 100 merchants
    { duration: '5m', target: 100 },   // Sustain 100 merchants
    { duration: '2m', target: 200 },   // Peak at 200
    { duration: '5m', target: 200 },   // Sustain peak
    { duration: '2m', target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<200'],
    http_req_failed: ['rate<0.02'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Simulate different merchant sessions
const merchants = Array.from({ length: 200 }, (_, i) => ({
  token: `merchant_${i}_token`,
  id: `merchant_${i}`,
}));

export default function () {
  const merchant = merchants[__VU % merchants.length];
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${merchant.token}`,
  };

  // Simulate merchant activity: view dashboard, check transactions
  const dashboardRes = http.get(`${BASE_URL}/api/v1/merchants/${merchant.id}/dashboard`, { headers });
  check(dashboardRes, {
    'dashboard status 200': (r) => r.status === 200,
  });

  sleep(1);

  const txRes = http.get(`${BASE_URL}/api/v1/merchants/${merchant.id}/transactions?limit=10`, { headers });
  check(txRes, {
    'transactions status 200': (r) => r.status === 200,
  });

  sleep(Math.random() * 3 + 1); // Random think time 1-4s
}
```

### Webhook Processing Test

```javascript
// load/webhook-processing.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

export const options = {
  stages: [
    { duration: '1m', target: 50 },
    { duration: '3m', target: 50 },
    { duration: '1m', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<100'],
    http_req_failed: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const WEBHOOK_SECRET = __ENV.WEBHOOK_SECRET || 'test-secret';

function generateWebhookPayload() {
  return JSON.stringify({
    event_type: 'subscription.created',
    data: {
      id: uuidv4(),
      customer_email: `user${__VU}@test.com`,
      plan_id: 'plan_123',
      status: 'active',
    },
  });
}

export default function () {
  const payload = generateWebhookPayload();
  // In real scenario, compute HMAC signature
  const headers = {
    'Content-Type': 'application/json',
    'X-Webhook-Signature': 'test-signature',
  };

  const res = http.post(`${BASE_URL}/webhooks/paddle`, payload, { headers });

  check(res, {
    'webhook accepted': (r) => r.status === 200 || r.status === 202,
    'processed quickly': (r) => r.timings.duration < 100,
  });

  sleep(0.5);
}
```

## Running Load Tests

```bash
# Run API latency test
k6 run load/api-latency.js \
  --env BASE_URL=http://localhost:8080 \
  --env AUTH_TOKEN=your-test-token

# Run dashboard load test
k6 run load/dashboard-load.js \
  --env BASE_URL=http://localhost:8080

# Run concurrent merchant test
k6 run load/concurrent-merchants.js \
  --env BASE_URL=http://localhost:8080

# Run webhook processing test
k6 run load/webhook-processing.js \
  --env BASE_URL=http://localhost:8080 \
  --env WEBHOOK_SECRET=your-secret

# Run with InfluxDB output
k6 run --out influxdb=http://localhost:8086/k6 load/api-latency.js

# Run with JSON summary
k6 run --summary-export=results.json load/api-latency.js
```

## Best Practices

1. **Baseline first** — Run tests against stable environment before optimization
2. **Realistic data** — Use production-like data volumes and distributions
3. **Gradual ramp-up** — Avoid sudden traffic spikes in tests
4. **Monitor infrastructure** — Watch CPU, memory, connections during tests
5. **Test in production-like environment** — Match production specs
6. **Automate regularly** — Run load tests weekly and before releases
7. **Compare results** — Track performance over time to detect regressions
