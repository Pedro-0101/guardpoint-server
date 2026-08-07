#!/bin/sh
set -e

MAX_RETRIES=5
RETRY_DELAY=3

echo "Running database migrations..."
for i in $(seq 1 $MAX_RETRIES); do
    if migrate -path /app/migrations -database "$DATABASE_URL" up; then
        echo "Migrations completed successfully"
        break
    fi
    if [ "$i" -eq "$MAX_RETRIES" ]; then
        echo "Migrations failed after $MAX_RETRIES attempts" >&2
        exit 1
    fi
    echo "Migration attempt $i failed, retrying in ${RETRY_DELAY}s..." >&2
    sleep $RETRY_DELAY
done

echo "Starting server..."
exec /app/server
