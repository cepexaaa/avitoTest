#!/bin/bash
set -e

until pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER"; do
  echo "Waiting for database connection..."
  sleep 2
done

echo "Applying database migrations..."
go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "user=$DB_USER password=$DB_PASSWORD dbname=$DB_NAME sslmode=disable host=$DB_HOST port=$DB_PORT" up
echo "Migrations applied successfully!"
exec ./server