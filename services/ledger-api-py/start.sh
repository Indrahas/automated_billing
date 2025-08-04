
#!/usr/bin/env bash
set -euo pipefail

# Generate stubs if missing
if [ ! -f ledger_pb2.py ]; then
  ./generate.sh
fi

# Start gRPC (background) and REST (foreground)
python grpc_server.py &
exec uvicorn app:app --host 0.0.0.0 --port ${LEDGER_API_HTTP_PORT:-8080}
