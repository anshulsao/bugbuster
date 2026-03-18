"""Catalog Service - FastAPI application with Redis caching, PostgreSQL, and OpenTelemetry."""

import json
import os
import uuid
from contextlib import asynccontextmanager
from decimal import Decimal

import psycopg2
import psycopg2.pool
import redis
import structlog
from fastapi import FastAPI, HTTPException, Request, Response
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.psycopg2 import Psycopg2Instrumentor
from opentelemetry.instrumentation.redis import RedisInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
PORT = int(os.getenv("PORT", "3002"))
DATABASE_URL = os.getenv("DATABASE_URL", "postgres://catalog_user:catalog_pass@catalog-db:5432/catalog")
REDIS_URL = os.getenv("REDIS_URL", "redis://redis:6379")
OTEL_ENDPOINT = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4318")
SERVICE_NAME = os.getenv("OTEL_SERVICE_NAME", "catalog-service")

CACHE_TTL = 60  # seconds

# ---------------------------------------------------------------------------
# Globals (initialised in lifespan)
# ---------------------------------------------------------------------------
db_pool: psycopg2.pool.ThreadedConnectionPool | None = None
redis_client: redis.Redis | None = None
tracer = trace.get_tracer(SERVICE_NAME)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
def _parse_dsn(url: str) -> dict:
    """Convert a postgres:// URL into kwargs for psycopg2."""
    from urllib.parse import urlparse

    parsed = urlparse(url)
    return {
        "host": parsed.hostname,
        "port": parsed.port or 5432,
        "dbname": parsed.path.lstrip("/"),
        "user": parsed.username,
        "password": parsed.password,
    }


class _DecimalEncoder(json.JSONEncoder):
    """Serialize Decimal values to float-safe strings."""

    def default(self, o):
        if isinstance(o, Decimal):
            return str(o)
        return super().default(o)


def _row_to_dict(row, cursor) -> dict:
    """Map a DB row tuple to a dict using cursor.description."""
    columns = [desc[0] for desc in cursor.description]
    d = dict(zip(columns, row))
    # Ensure JSON-safe types
    for k, v in d.items():
        if isinstance(v, Decimal):
            d[k] = str(v)
        elif hasattr(v, "isoformat"):
            d[k] = v.isoformat()
    return d


# ---------------------------------------------------------------------------
# Structured logging
# ---------------------------------------------------------------------------
def _setup_logging():
    structlog.configure(
        processors=[
            structlog.contextvars.merge_contextvars,
            structlog.processors.add_log_level,
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.processors.JSONRenderer(),
        ],
        wrapper_class=structlog.make_filtering_bound_logger(0),
        context_class=dict,
        logger_factory=structlog.PrintLoggerFactory(),
        cache_logger_on_first_use=True,
    )


# ---------------------------------------------------------------------------
# OpenTelemetry
# ---------------------------------------------------------------------------
def _setup_telemetry():
    resource = Resource.create({"service.name": SERVICE_NAME})
    provider = TracerProvider(resource=resource)
    exporter = OTLPSpanExporter(endpoint=f"{OTEL_ENDPOINT}/v1/traces")
    provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)

    # Auto-instrument libraries
    Psycopg2Instrumentor().instrument()
    RedisInstrumentor().instrument()


# ---------------------------------------------------------------------------
# Lifespan (startup / shutdown)
# ---------------------------------------------------------------------------
@asynccontextmanager
async def lifespan(app: FastAPI):
    global db_pool, redis_client
    log = structlog.get_logger()

    # --- Startup ---
    _setup_logging()
    _setup_telemetry()

    # PostgreSQL connection pool
    dsn = _parse_dsn(DATABASE_URL)
    try:
        db_pool = psycopg2.pool.ThreadedConnectionPool(minconn=2, maxconn=10, **dsn)
        log.info("postgres_connected", host=dsn["host"], db=dsn["dbname"])
    except Exception as exc:
        log.error("postgres_connect_failed", error=str(exc))

    # Redis
    try:
        redis_client = redis.Redis.from_url(REDIS_URL, decode_responses=True)
        redis_client.ping()
        log.info("redis_connected", url=REDIS_URL)
    except Exception as exc:
        log.warning("redis_connect_failed", error=str(exc))
        redis_client = None

    yield

    # --- Shutdown ---
    if db_pool:
        db_pool.closeall()
        log.info("postgres_disconnected")
    if redis_client:
        redis_client.close()
        log.info("redis_disconnected")


# ---------------------------------------------------------------------------
# FastAPI app
# ---------------------------------------------------------------------------
app = FastAPI(title="Catalog Service", version="1.0.0", lifespan=lifespan)

# Instrument FastAPI after app creation
FastAPIInstrumentor.instrument_app(app)


# ---------------------------------------------------------------------------
# Middleware — request-id + trace-id correlation
# ---------------------------------------------------------------------------
@app.middleware("http")
async def request_context(request: Request, call_next):
    request_id = request.headers.get("x-request-id", str(uuid.uuid4()))
    span = trace.get_current_span()
    trace_id = format(span.get_span_context().trace_id, "032x") if span and span.get_span_context().is_valid else ""

    structlog.contextvars.clear_contextvars()
    structlog.contextvars.bind_contextvars(request_id=request_id, trace_id=trace_id)

    log = structlog.get_logger()
    log.info("request_started", method=request.method, path=request.url.path)

    response: Response = await call_next(request)

    response.headers["X-Request-ID"] = request_id
    if trace_id:
        response.headers["X-Trace-ID"] = trace_id

    log.info("request_completed", method=request.method, path=request.url.path, status=response.status_code)
    return response


# ---------------------------------------------------------------------------
# Routes
# ---------------------------------------------------------------------------
@app.get("/health")
async def health():
    return {"status": "ok"}


@app.get("/api/catalog/products")
async def list_products():
    """Return all products. Attempts Redis cache first, falls back to PostgreSQL."""
    log = structlog.get_logger()
    cache_key = "catalog:products:all"

    # --- Try cache ---
    if redis_client:
        try:
            cached = redis_client.get(cache_key)
            if cached:
                log.info("cache_hit", key=cache_key)
                return json.loads(cached)
        except Exception as exc:
            log.warning("cache_read_error", error=str(exc))

    # --- Fallback to DB ---
    if not db_pool:
        raise HTTPException(status_code=503, detail="Database unavailable")

    conn = db_pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT id, name, description, price, stock, category, created_at FROM products ORDER BY id")
            rows = cur.fetchall()
            products = [_row_to_dict(r, cur) for r in rows]
    except Exception as exc:
        log.error("db_query_failed", error=str(exc))
        raise HTTPException(status_code=500, detail="Internal server error")
    finally:
        db_pool.putconn(conn)

    # --- Populate cache ---
    if redis_client:
        try:
            redis_client.setex(cache_key, CACHE_TTL, json.dumps(products, cls=_DecimalEncoder))
            log.info("cache_set", key=cache_key, ttl=CACHE_TTL)
        except Exception as exc:
            log.warning("cache_write_error", error=str(exc))

    return products


@app.get("/api/catalog/products/{product_id}")
async def get_product(product_id: int):
    """Return a single product by ID. Attempts Redis cache first, falls back to PostgreSQL."""
    log = structlog.get_logger()
    cache_key = f"catalog:products:{product_id}"

    # --- Try cache ---
    if redis_client:
        try:
            cached = redis_client.get(cache_key)
            if cached:
                log.info("cache_hit", key=cache_key)
                return json.loads(cached)
        except Exception as exc:
            log.warning("cache_read_error", error=str(exc))

    # --- Fallback to DB ---
    if not db_pool:
        raise HTTPException(status_code=503, detail="Database unavailable")

    conn = db_pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT id, name, description, price, stock, category, created_at FROM products WHERE id = %s",
                (product_id,),
            )
            row = cur.fetchone()
            if not row:
                raise HTTPException(status_code=404, detail="Product not found")
            product = _row_to_dict(row, cur)
    except HTTPException:
        raise
    except Exception as exc:
        log.error("db_query_failed", error=str(exc))
        raise HTTPException(status_code=500, detail="Internal server error")
    finally:
        db_pool.putconn(conn)

    # --- Populate cache ---
    if redis_client:
        try:
            redis_client.setex(cache_key, CACHE_TTL, json.dumps(product, cls=_DecimalEncoder))
            log.info("cache_set", key=cache_key, ttl=CACHE_TTL)
        except Exception as exc:
            log.warning("cache_write_error", error=str(exc))

    return product


# ---------------------------------------------------------------------------
# Entrypoint (for direct execution)
# ---------------------------------------------------------------------------
if __name__ == "__main__":
    import uvicorn

    uvicorn.run("app.main:app", host="0.0.0.0", port=PORT, log_level="info")
