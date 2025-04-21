#!/bin/bash

# Connect to fly.io postgres
fly postgres connect -a your-db-app-name << EOF

-- Put your SQL migrations here
ALTER TABLE your_table ADD COLUMN new_column VARCHAR(255);

-- Add more migrations as needed

EOF

echo "Database migration completed!" 