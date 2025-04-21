#!/bin/bash

# Check if the application is healthy
check_health() {
    response=$(curl -s https://unicorn-app-backend.fly.dev/health)
    if [[ $response == *"ok"* ]]; then
        return 0
    else
        return 1
    fi
}

# Deploy and verify
fly deploy

# Wait for deployment to stabilize
sleep 10

# Check health
if check_health; then
    echo "Deployment successful!"
else
    echo "Deployment might have issues, rolling back..."
    ./rollback.sh
    exit 1
fi 