'use strict';

const express = require('express');
const { Pool } = require('pg');
const amqplib = require('amqplib');
const axios = require('axios');
const pino = require('pino');
const crypto = require('crypto');

// ---------------------------------------------------------------------------
// Logger
// ---------------------------------------------------------------------------
const logger = pino({
  level: process.env.LOG_LEVEL || 'info',
  formatters: {
    level(label) {
      return { level: label };
    },
  },
  timestamp: pino.stdTimeFunctions.isoTime,
});

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------
const PORT = parseInt(process.env.PORT, 10) || 3001;
const DATABASE_URL = process.env.DATABASE_URL || 'postgres://orders_user:orders_pass@localhost:5432/orders';
const RABBITMQ_URL = process.env.RABBITMQ_URL || 'amqp://bugbuster:bugbuster@localhost:5672';
const CATALOG_SERVICE_URL = process.env.CATALOG_SERVICE_URL || 'http://localhost:3002';
const PAYMENT_SERVICE_URL = process.env.PAYMENT_SERVICE_URL || 'http://localhost:3003';

// ---------------------------------------------------------------------------
// Postgres
// ---------------------------------------------------------------------------
const pool = new Pool({ connectionString: DATABASE_URL });

pool.on('error', (err) => {
  logger.error({ err }, 'Unexpected PostgreSQL pool error');
});

// ---------------------------------------------------------------------------
// RabbitMQ
// ---------------------------------------------------------------------------
let rabbitChannel = null;
const EXCHANGE = 'orders';

async function connectRabbit() {
  try {
    const conn = await amqplib.connect(RABBITMQ_URL);
    conn.on('error', (err) => {
      logger.error({ err }, 'RabbitMQ connection error');
      rabbitChannel = null;
    });
    conn.on('close', () => {
      logger.warn('RabbitMQ connection closed, will retry');
      rabbitChannel = null;
      setTimeout(connectRabbit, 5000);
    });
    const ch = await conn.createChannel();
    await ch.assertExchange(EXCHANGE, 'topic', { durable: true });
    rabbitChannel = ch;
    logger.info('Connected to RabbitMQ');
  } catch (err) {
    logger.error({ err }, 'Failed to connect to RabbitMQ, retrying in 5s');
    setTimeout(connectRabbit, 5000);
  }
}

function publishEvent(routingKey, payload) {
  if (!rabbitChannel) {
    logger.warn({ routingKey }, 'RabbitMQ channel not available, event dropped');
    return false;
  }
  rabbitChannel.publish(
    EXCHANGE,
    routingKey,
    Buffer.from(JSON.stringify(payload)),
    { persistent: true, contentType: 'application/json' },
  );
  return true;
}

// ---------------------------------------------------------------------------
// Express app
// ---------------------------------------------------------------------------
const app = express();
app.use(express.json());

// Attach request_id to every request
app.use((req, _res, next) => {
  req.requestId = req.headers['x-request-id'] || crypto.randomUUID();
  req.log = logger.child({ request_id: req.requestId });
  next();
});

// Request logging
app.use((req, res, next) => {
  const start = Date.now();
  res.on('finish', () => {
    req.log.info({
      method: req.method,
      url: req.originalUrl,
      status: res.statusCode,
      duration_ms: Date.now() - start,
    }, 'request completed');
  });
  next();
});

// ---- Health ---------------------------------------------------------------
app.get('/health', (_req, res) => {
  res.json({ status: 'ok' });
});

// ---- List orders ----------------------------------------------------------
app.get('/api/orders', async (req, res) => {
  try {
    const result = await pool.query(
      'SELECT id, product_id, quantity, total_price, status, payment_id, created_at FROM orders ORDER BY created_at DESC',
    );
    res.json(result.rows);
  } catch (err) {
    req.log.error({ err }, 'Failed to list orders');
    res.status(500).json({ error: 'Internal server error' });
  }
});

// ---- Get single order -----------------------------------------------------
app.get('/api/orders/:id', async (req, res) => {
  try {
    const { id } = req.params;
    const result = await pool.query(
      'SELECT id, product_id, quantity, total_price, status, payment_id, created_at FROM orders WHERE id = $1',
      [id],
    );
    if (result.rows.length === 0) {
      return res.status(404).json({ error: 'Order not found' });
    }
    res.json(result.rows[0]);
  } catch (err) {
    req.log.error({ err }, 'Failed to get order');
    res.status(500).json({ error: 'Internal server error' });
  }
});

// ---- Create order ---------------------------------------------------------
app.post('/api/orders', async (req, res) => {
  const { product_id, quantity } = req.body;

  // Validate input
  if (product_id == null || quantity == null) {
    return res.status(400).json({ error: 'product_id and quantity are required' });
  }
  if (!Number.isInteger(product_id) || product_id <= 0) {
    return res.status(400).json({ error: 'product_id must be a positive integer' });
  }
  if (!Number.isInteger(quantity) || quantity <= 0) {
    return res.status(400).json({ error: 'quantity must be a positive integer' });
  }

  try {
    // 1. Fetch product price from Catalog Service
    req.log.info({ product_id }, 'Fetching product from catalog');
    const catalogRes = await axios.get(`${CATALOG_SERVICE_URL}/api/catalog/products/${product_id}`, {
      headers: { 'X-Request-ID': req.requestId },
      timeout: 5000,
    });
    const product = catalogRes.data;
    const totalPrice = (parseFloat(product.price) * quantity).toFixed(2);

    // 2. Insert order into DB with status "pending"
    const insertResult = await pool.query(
      `INSERT INTO orders (product_id, quantity, total_price, status, created_at)
       VALUES ($1, $2, $3, 'pending', NOW())
       RETURNING id, product_id, quantity, total_price, status, payment_id, created_at`,
      [product_id, quantity, totalPrice],
    );
    const order = insertResult.rows[0];
    req.log.info({ order_id: order.id }, 'Order created with status pending');

    // 3. Call Payment Service
    req.log.info({ order_id: order.id, total_price: totalPrice }, 'Processing payment');
    let paymentResult;
    try {
      const paymentRes = await axios.post(
        `${PAYMENT_SERVICE_URL}/api/payments`,
        { order_id: order.id, amount: parseFloat(totalPrice) },
        { headers: { 'X-Request-ID': req.requestId }, timeout: 10000 },
      );
      paymentResult = paymentRes.data;
    } catch (payErr) {
      req.log.error({ err: payErr.message, order_id: order.id }, 'Payment failed');
      // Mark order as payment_failed
      await pool.query("UPDATE orders SET status = 'payment_failed' WHERE id = $1", [order.id]);
      order.status = 'payment_failed';
      return res.status(502).json({ error: 'Payment processing failed', order });
    }

    // 4. Update order with payment info
    const updateResult = await pool.query(
      `UPDATE orders SET status = 'confirmed', payment_id = $1 WHERE id = $2
       RETURNING id, product_id, quantity, total_price, status, payment_id, created_at`,
      [paymentResult.payment_id || paymentResult.id, order.id],
    );
    const confirmedOrder = updateResult.rows[0];

    // 5. Publish event to RabbitMQ
    const published = publishEvent('order.created', {
      order_id: confirmedOrder.id,
      product_id: confirmedOrder.product_id,
      quantity: confirmedOrder.quantity,
      total_price: confirmedOrder.total_price,
      status: confirmedOrder.status,
      payment_id: confirmedOrder.payment_id,
      created_at: confirmedOrder.created_at,
    });
    req.log.info({ order_id: confirmedOrder.id, published }, 'Order confirmed, event published');

    res.status(201).json(confirmedOrder);
  } catch (err) {
    if (err.response && err.response.status === 404) {
      return res.status(404).json({ error: 'Product not found' });
    }
    req.log.error({ err: err.message }, 'Failed to create order');
    res.status(500).json({ error: 'Internal server error' });
  }
});

// ---------------------------------------------------------------------------
// Start server
// ---------------------------------------------------------------------------
const server = app.listen(PORT, () => {
  logger.info({ port: PORT }, 'Order service started');
});

connectRabbit();

// ---------------------------------------------------------------------------
// Graceful shutdown
// ---------------------------------------------------------------------------
function shutdown(signal) {
  logger.info({ signal }, 'Shutting down gracefully');
  server.close(async () => {
    try {
      if (rabbitChannel) await rabbitChannel.close();
    } catch (_) { /* ignore */ }
    try {
      await pool.end();
    } catch (_) { /* ignore */ }
    logger.info('Cleanup complete, exiting');
    process.exit(0);
  });

  // Force exit after 10s
  setTimeout(() => {
    logger.warn('Forced shutdown after timeout');
    process.exit(1);
  }, 10000);
}

process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);
