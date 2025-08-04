
.PHONY: up down build proto db

up:
	docker compose up --build

down:
	docker compose down -v

build:
	(cd services/billing-worker-go && go build ./...)
	(cd services/outbox-publisher-go && go build ./...)

proto:
	bash services/ledger-api-py/generate.sh

db:
	psql postgresql://postgres:postgres@localhost:5432/billing -f db/schema.sql
