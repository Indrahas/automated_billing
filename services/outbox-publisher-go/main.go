
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

func main() {
	kafkaBrokers := getenv("KAFKA_BROKERS", "localhost:9092")
	eventsTopic := getenv("KAFKA_BILLING_EVENTS_TOPIC", "billing.events")
	pgDSN := getenv("PG_DSN", "postgres://postgres:postgres@localhost:5432/billing?sslmode=disable")

	log.Printf("Starting outbox publisher: brokers=%s topic=%s", kafkaBrokers, eventsTopic)

	pool, err := pgxpool.New(context.Background(), pgDSN)
	if err != nil {
		log.Fatalf("pg connect failed: %v", err)
	}
	defer pool.Close()

	w := &kafka.Writer{
		Addr:     kafka.TCP(kafkaBrokers),
		Topic:    eventsTopic,
		Balancer: &kafka.LeastBytes{},
	}

	defer w.Close()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if err := publishBatch(context.Background(), pool, w); err != nil {
			log.Printf("publish error: %v", err)
			time.Sleep(time.Second)
		}
	}
}

func publishBatch(ctx context.Context, pool *pgxpool.Pool, w *kafka.Writer) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		select id, payload_json::text from outbox
		where published_at is null
		order by id
		for update skip locked
		limit 100
	`)
	if err != nil {
		return err
	}
	type item struct{ id int64; payload string }
	var items []item
	for rows.Next() {
		var id int64
		var payload string
		if err := rows.Scan(&id, &payload); err != nil {
			return err
		}
		items = append(items, item{id, payload})
	}
	rows.Close()

	if len(items) == 0 {
		return nil
	}

	msgs := make([]kafka.Message, 0, len(items))
	for _, it := range items {
		msgs = append(msgs, kafka.Message{Value: []byte(it.payload)})
	}

	if err := w.WriteMessages(ctx, msgs...); err != nil {
		return err
	}

	// mark as published
	_, err = tx.Exec(ctx, `
		update outbox set published_at = now()
		where id = any($1)
	`, ids(items))
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func ids(items []item) []int64 {
	out := make([]int64, 0, len(items))
	for _, it := range items {
		out = append(out, it.id)
	}
	return out
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
