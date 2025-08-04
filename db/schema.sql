
-- Core billing schema

create table if not exists invoices (
  id bigserial primary key,
  order_id text not null,
  currency text not null,
  subtotal numeric(18,2) not null,
  tax numeric(18,2) not null,
  total numeric(18,2) not null,
  status text not null default 'CREATED',
  idempotency_key text not null unique,
  version int not null default 1,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists invoice_lines (
  id bigserial primary key,
  invoice_id bigint not null references invoices(id) on delete cascade,
  sku text not null,
  qty int not null,
  unit_price numeric(18,2) not null,
  tax_rate numeric(9,4) not null default 0.18
);

create table if not exists ledger_entries (
  id bigserial primary key,
  txn_id uuid not null,
  account_dr text not null,
  account_cr text not null,
  amount numeric(18,2) not null,
  currency text not null,
  created_at timestamptz not null default now()
);

-- Outbox pattern for exactly-once publishing
create table if not exists outbox (
  id bigserial primary key,
  aggregate text not null,
  event_type text not null,
  payload_json jsonb not null,
  published_at timestamptz
);

create index if not exists idx_outbox_unpublished on outbox (published_at);

-- Simple invariant check view (optional)
create or replace view v_ledger_invariants as
select txn_id, sum(case when account_dr <> '' then amount else 0 end) as total_debits,
       sum(case when account_cr <> '' then amount else 0 end) as total_credits
from ledger_entries
group by txn_id;
