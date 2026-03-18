import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const TARGET_URL = __ENV.TARGET_URL || 'http://localhost:80';
const RATE = parseInt(__ENV.RATE || '20', 10);

export const options = {
    scenarios: {
        traffic: {
            executor: 'constant-vus',
            vus: RATE,
            duration: '60m',
        },
    },
    thresholds: {
        http_req_duration: ['p(95)<2000'],
    },
};

const errorRate = new Rate('errors');

function browseCatalog() {
    const res = http.get(`${TARGET_URL}/api/catalog/products`);
    check(res, {
        'catalog status 200': (r) => r.status === 200,
    });
    errorRate.add(res.status !== 200);
}

function createOrder() {
    const productId = Math.floor(Math.random() * 10) + 1;
    const quantity = Math.floor(Math.random() * 5) + 1;

    const payload = JSON.stringify({
        product_id: productId,
        quantity: quantity,
    });

    const params = {
        headers: {
            'Content-Type': 'application/json',
        },
    };

    const res = http.post(`${TARGET_URL}/api/orders`, payload, params);
    check(res, {
        'order created': (r) => r.status === 200 || r.status === 201,
    });
    errorRate.add(res.status !== 200 && res.status !== 201);
}

function getOrder() {
    const orderId = Math.floor(Math.random() * 100) + 1;
    const res = http.get(`${TARGET_URL}/api/orders/${orderId}`);
    check(res, {
        'order retrieved': (r) => r.status === 200 || r.status === 404,
    });
    errorRate.add(res.status >= 500);
}

export default function () {
    const rand = Math.random();

    if (rand < 0.7) {
        browseCatalog();
    } else if (rand < 0.9) {
        createOrder();
    } else {
        getOrder();
    }

    // Random sleep between 100-500ms
    sleep(0.1 + Math.random() * 0.4);
}
