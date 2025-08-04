
import os
import time
from concurrent import futures

import grpc
import psycopg2
from dotenv import load_dotenv

import ledger_pb2
import ledger_pb2_grpc

load_dotenv()

PG_CONN = dict(
    host=os.getenv("PG_HOST", "localhost"),
    port=int(os.getenv("PG_PORT", "5432")),
    user=os.getenv("PG_USER", "postgres"),
    password=os.getenv("PG_PASSWORD", "postgres"),
    dbname=os.getenv("PG_DATABASE", "billing"),
)

class LedgerService(ledger_pb2_grpc.LedgerServiceServicer):
    def CreateEntry(self, request, context):
        e = request.entry
        try:
            conn = psycopg2.connect(**PG_CONN)
            cur = conn.cursor()
            cur.execute("""
                insert into ledger_entries (txn_id, account_dr, account_cr, amount, currency)
                values (%s, %s, %s, %s::numeric, %s)
            """, (e.txn_id, e.account_dr, e.account_cr, e.amount, e.currency))
            conn.commit()
            cur.close()
            conn.close()
            return ledger_pb2.CreateEntryResponse(ok=True, message="inserted")
        except Exception as ex:
            context.set_details(str(ex))
            context.set_code(grpc.StatusCode.INTERNAL)
            return ledger_pb2.CreateEntryResponse(ok=False, message=str(ex))

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    ledger_pb2_grpc.add_LedgerServiceServicer_to_server(LedgerService(), server)
    port = os.getenv("LEDGER_API_GRPC_PORT", "50051")
    server.add_insecure_port(f'[::]:{port}')
    server.start()
    print(f"gRPC server started on {port}")
    try:
        while True:
            time.sleep(86400)
    except KeyboardInterrupt:
        server.stop(0)

if __name__ == "__main__":
    serve()
