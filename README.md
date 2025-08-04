
# Automated Billing

This project demonstrates an **event-driven billing system** that consumes order events, computes invoices (with taxes, FX hooks), writes **double-entry ledger** records, and emits downstream events using the **outbox pattern** for exactly-once semantics. It includes a **Ledger API** (REST + gRPC) and nightly CSV exports to S3.


---

## Local Development

### Prerequisites
- Docker & Docker Compose
- Make (optional)
- Python 3.10+ and Go 1.21+ if running services outside Docker

### Quickstart (Docker Compose)
```bash
# 1) Copy environment variables and set values
cp .env.example .env

# 2) Start infra + services (Kafka, Postgres, Ledger API, Worker, Outbox Publisher)
docker compose up --build

# 3) Produce some sample orders
python tools/order-producer-py/producer.py
```

### Verify
- **Postgres** at `localhost:5432` (user: postgres, password: postgres, db: billing)
- **Ledger REST API** at http://localhost:8080/docs
- **gRPC server** on `localhost:50051` (see `proto/ledger.proto`)
- **Kafka** at `localhost:9092` (auto-create topics enabled)

### Useful commands
```bash
# Apply schema (compose does it automatically, but you can run manually):
psql postgresql://postgres:postgres@localhost:5432/billing -f db/schema.sql

# Regenerate Python gRPC stubs:
bash services/ledger-api-py/generate.sh

# Lint Go modules:
(cd services/billing-worker-go && go mod tidy && go build)
(cd services/outbox-publisher-go && go mod tidy && go build)
```

---

## Kubernetes (Minikube or EKS)
Manifests in `deploy/k8s/` include Deployments, Services, and a Postgres StatefulSet. For AWS:
- Use **MSK** for Kafka, **RDS** for Postgres, and **S3** for exports.
- See `infra/terraform/` for a starting Terraform skeleton (fill in your VPC, subnets, and secrets).

Apply to a cluster (local example with Minikube and an external Kafka endpoint):
```bash
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/postgres.yaml
kubectl apply -f deploy/k8s/ledger-api.yaml
kubectl apply -f deploy/k8s/billing-worker.yaml
kubectl apply -f deploy/k8s/outbox-publisher.yaml
```

---

## Project Structure
```
db/schema.sql
proto/ledger.proto
services/
  billing-worker-go/        # Consumes 'orders', writes invoices/ledger/outbox
  outbox-publisher-go/      # Publishes from outbox -> Kafka 'billing.events'
  ledger-api-py/            # REST (FastAPI) + gRPC server; CSV exports to S3
deploy/k8s/                 # Kubernetes manifests
infra/terraform/            # AWS skeleton for MSK, RDS, S3
tools/order-producer-py/    # Local producer to send order events
```

```

