
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

type OrderLine struct {
	SKU       string  `json:"sku"`
	Qty       int     `json:"qty"`
	UnitPrice float64 `json:"unit_price"`
}

type OrderEvent struct {
	EventID   string      `json:"event_id"`
	OrderID   string      `json:"order_id"`
	Currency  string      `json:"currency"`
	Lines     []OrderLine `json:"lines"`
	Timestamp time.Time   `json:"timestamp"`
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func main() {
	kafkaBrokers := getenv("KAFKA_BROKERS", "localhost:9092")
	ordersTopic := getenv("KAFKA_ORDERS_TOPIC", "orders")
	groupID := getenv("KAFKA_GROUP_ID", "billing-worker")
	pgDSN := getenv("PG_DSN", "postgres://postgres:postgres@localhost:5432/billing?sslmode=disable")

	log.Printf("Starting billing worker: brokers=%s topic=%s pg=%s", kafkaBrokers, ordersTopic, pgDSN)

	pool, err := pgxpool.New(context.Background(), pgDSN)
	if err != nil {
		log.Fatalf("pg connect failed: %v", err)
	}
	defer pool.Close()

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{kafkaBrokers},
		Topic:    ordersTopic,
		GroupID:  groupID,
		MinBytes: 1e3,
		MaxBytes: 1e6,
	})

	defer r.Close()

	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			log.Printf("kafka read error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		var evt OrderEvent
		if err := json.Unmarshal(m.Value, &evt); err != nil {
			log.Printf("invalid order json: %v", err)
			continue
		}

		if evt.EventID == "" {
			log.Printf("missing event_id")
			continue
		}

		if err := processOrder(context.Background(), pool, &evt); err != nil {
			log.Printf("process error for event %s: %v", evt.EventID, err)
			continue
		}
	}
}

func processOrder(ctx context.Context, pool *pgxpool.Pool, evt *OrderEvent) error {
	// Compute totals
	var subtotal float64
	for _, ln := range evt.Lines {
		subtotal += float64(ln.Qty) * ln.UnitPrice
	}
	tax := round2(subtotal * 0.18) // demo: 18% tax
	total := round2(subtotal + tax)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Idempotency via event_id
	var invoiceID int64
	err = tx.QueryRow(ctx, `
		insert into invoices (order_id, currency, subtotal, tax, total, status, idempotency_key)
		values ($1,$2,$3,$4,$5,'CREATED',$6)
		on conflict (idempotency_key) do update set updated_at=now()
		returning id
	`, evt.OrderID, evt.Currency, round2(subtotal), tax, total, evt.EventID).Scan(&invoiceID)
	if err != nil {
		return fmt.Errorf("insert invoice: %w", err)
	}

	// Insert lines (naive approach; in production you'd upsert or diff)
	_, err = tx.Exec(ctx, `delete from invoice_lines where invoice_id=$1`, invoiceID)
	if err != nil {
		return fmt.Errorf("clear lines: %w", err)
	}

	for _, ln := range evt.Lines {
		_, err := tx.Exec(ctx, `
			insert into invoice_lines (invoice_id, sku, qty, unit_price, tax_rate)
			values ($1,$2,$3,$4,$5)
		`, invoiceID, ln.SKU, ln.Qty, ln.UnitPrice, 0.18)
		if err != nil {
			return fmt.Errorf("insert line: %w", err)
		}
	}

	// Ledger: simple double-entry (AR vs Revenue+Tax)
	txnID := uuid.New().String()
	_, err = tx.Exec(ctx, `
		insert into ledger_entries (txn_id, account_dr, account_cr, amount, currency)
		values ($1,$2,$3,$4,$5),
		       ($1,$6,$7,$8,$5)
	`, txnID,
		"Accounts Receivable", "Revenue", round2(subtotal), evt.Currency,
		"Accounts Receivable", "Tax Payable", tax)
	if err != nil {
		return fmt.Errorf("ledger entries: %w", err)
	}

	// Outbox publish (invoice_created)
	payload := map[string]any{
		"type":       "invoice_created",
		"invoice_id": invoiceID,
		"order_id":   evt.OrderID,
		"total":      total,
		"currency":   evt.Currency,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	b, _ := json.Marshal(payload)
	_, err = tx.Exec(ctx, `
		insert into outbox (aggregate, event_type, payload_json)
		values ($1,$2,$3)
	`, "invoice", "invoice_created", string(b))
	if err != nil {
		return fmt.Errorf("outbox insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	log.Printf("Processed order %s -> invoice %d total=%.2f %s", evt.OrderID, invoiceID, total, evt.Currency)
	return nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
