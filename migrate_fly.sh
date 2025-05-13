#!/bin/bash

# Set the database URL directly
DB_URL="postgres://unicorn_app_backend:254EHPdfcfusqtO@localhost:5433/unicorn_app_backend?sslmode=disable"

if [ -z "$DB_URL" ]; then
    echo "Failed to get database URL"
    exit 1
fi

# Run the schema
psql "$DB_URL" << EOF
$(cat db/schema.go | sed -n '/^const Schema = `/,/^`/p' | sed '1d' | sed '$d')
EOF

echo "Schema migration completed!" 