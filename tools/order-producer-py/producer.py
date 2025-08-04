
import json, os, time, uuid, random, datetime as dt
from kafka import KafkaProducer

brokers = os.getenv("KAFKA_BROKERS", "localhost:9092")
topic = os.getenv("KAFKA_ORDERS_TOPIC", "orders")

producer = KafkaProducer(bootstrap_servers=brokers.split(","),
                         value_serializer=lambda v: json.dumps(v).encode("utf-8"))

SKUS = ["PRO", "BASIC", "ADDON_X", "ADDON_Y"]

def random_order():
    lines = []
    for _ in range(random.randint(1, 3)):
        sku = random.choice(SKUS)
        qty = random.randint(1, 5)
        price = random.choice([9.99, 19.99, 49.00, 99.00])
        lines.append({"sku": sku, "qty": qty, "unit_price": price})
    return {
        "event_id": str(uuid.uuid4()),
        "order_id": str(uuid.uuid4()),
        "currency": "USD",
        "lines": lines,
        "timestamp": dt.datetime.utcnow().isoformat()
    }

if __name__ == "__main__":
    print(f"Producing orders to {topic} on {brokers}... Ctrl+C to stop")
    try:
        while True:
            evt = random_order()
            producer.send(topic, evt)
            print("sent:", evt["order_id"], "lines:", len(evt["lines"]))
            time.sleep(1.0)
    except KeyboardInterrupt:
        pass
    finally:
        producer.flush()
        producer.close()
