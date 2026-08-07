#!/bin/sh
set -e

echo "Running database migrations..."
migrate -path /app/migrations -database "$DATABASE_URL" up

echo "Starting server..."
exec /app/server
