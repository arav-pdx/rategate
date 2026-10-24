import http from 'k6/http';
import { check } from 'k6';

export const options = {
  scenarios: {
    constant_request_rate: {
      executor: 'constant-arrival-rate',
      rate: 50,
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 50,
      maxVUs: 100,
    },
  },
};

const TOKEN = __ENV.TOKEN;

export default function () {
  const params = {
    headers: {
      'Authorization': `Bearer ${TOKEN}`,
      'Content-Type': 'application/json',
    },
  };

  const res = http.get('http://localhost:8080/api/orders', params);
  check(res, {
    'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
  });
}
