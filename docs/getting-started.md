# Getting Started

Gebeta Billing requires a few infrastructure components to operate correctly: **PostgreSQL**, **Redis**, and **RabbitMQ**. 

Because of these dependencies, **deploying via Docker Compose is the recommended and easiest method**. It spins up the Gebeta Billing application alongside all its required databases and queues in a single, isolated environment.

---

## Installation Steps

### 1. Requirements
*   [Docker](https://docs.docker.com/get-docker/) installed.
*   [Docker Compose](https://docs.docker.com/compose/install/) installed.

### 2. Download Docker Compose File
Grab the latest `docker-compose.yaml` from the repository. You can download it directly:

```bash
curl -O https://raw.githubusercontent.com/thec1oud/billing/main/docker-compose.yaml
```

*(Note: In a production environment, you should use `docker-compose.prod.yaml` which lacks debug ports and development overrides).*

### 3. Configuration
The Docker Compose file relies on an `.env` file for configuration. Create a new file named `.env` in the same directory:

```bash
# App Configuration
APP_ENV=development
APP_PORT=8080
LOG_TARGETS=stdout

# Database (PostgreSQL)
DB_HOST=postgres
DB_PORT=5432
DB_USER=billing_user
DB_PASSWORD=billing_password
DB_NAME=billing_db

# Cache & Locking (Redis)
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=

# Event Broker (RabbitMQ)
RABBITMQ_HOST=rabbitmq
RABBITMQ_PORT=5672
RABBITMQ_MANAGEMENT_PORT=15672
RABBITMQ_DEFAULT_USER=rabbit_user
RABBITMQ_DEFAULT_PASS=rabbit_password
```

> [!NOTE] 
> The `DB_HOST`, `REDIS_HOST`, and `RABBITMQ_HOST` values should match the service names defined in your `docker-compose.yaml` (as shown above).

### 4. Start the System
Run the following command in the directory containing your `docker-compose.yaml` and `.env` files:

```bash
docker compose up -d
```

**What happens next?**
1. Docker will download the necessary images for PostgreSQL, Redis, RabbitMQ, and the Gebeta Billing App.
2. The databases will initialize.
3. Once the Gebeta Billing app connects to PostgreSQL, **it will automatically run all necessary database migrations.** There are no manual SQL scripts to run.
4. The system will be available at `http://localhost:8080`.

---

## Verifying the Installation

To verify the system is running correctly, you can hit the health check endpoint:

```bash
curl http://localhost:8080/health
```

If everything is wired up correctly, you should receive a 200 OK response indicating that the application is successfully connected to PostgreSQL, Redis, and RabbitMQ.
