
import os
import csv
import io
import json
import datetime as dt
from typing import List, Optional

import psycopg2
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import boto3

PG_CONN = dict(
    host=os.getenv("PG_HOST", "localhost"),
    port=int(os.getenv("PG_PORT", "5432")),
    user=os.getenv("PG_USER", "postgres"),
    password=os.getenv("PG_PASSWORD", "postgres"),
    dbname=os.getenv("PG_DATABASE", "billing"),
)

app = FastAPI(title="Ledger API", version="0.1.0")

def get_conn():
    return psycopg2.connect(**PG_CONN)

class ExportResponse(BaseModel):
    url: Optional[str] = None
    rows: int

@app.get("/healthz")
def healthz():
    return {"ok": True}

@app.get("/exports/daily", response_model=ExportResponse)
def export_daily():
    """Export yesterday's invoices and ledger entries to CSV and upload to S3 (if configured)."""
    date_from = (dt.datetime.utcnow().date() - dt.timedelta(days=1))
    date_to = date_from + dt.timedelta(days=1)
    conn = get_conn()
    cur = conn.cursor()

    cur.execute("""
        select id, order_id, currency, subtotal, tax, total, created_at
        from invoices
        where created_at >= %s and created_at < %s
        order by id
    """, (date_from, date_to))
    rows = cur.fetchall()

    output = io.StringIO()
    w = csv.writer(output)
    w.writerow(["id","order_id","currency","subtotal","tax","total","created_at"])
    for r in rows:
        w.writerow(r)

    key_name = f"exports/invoices_{date_from.isoformat()}.csv"
    data = output.getvalue().encode("utf-8")

    url = None
    bucket = os.getenv("AWS_S3_BUCKET")
    if bucket:
        s3 = boto3.client("s3", region_name=os.getenv("AWS_REGION", "ap-south-1"))
        s3.put_object(Bucket=bucket, Key=key_name, Body=data, ContentType="text/csv")
        url = f"s3://{bucket}/{key_name}"
    else:
        # local fallback
        os.makedirs("/app/exports", exist_ok=True)
        with open(f"/app/exports/{os.path.basename(key_name)}", "wb") as f:
            f.write(data)
        url = f"file:///app/exports/{os.path.basename(key_name)}"

    cur.close()
    conn.close()
    return {"url": url, "rows": len(rows)}
