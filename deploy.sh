#!/bin/bash

# Check if a commit message was provided
if [ -z "$1" ]; then
    echo "Please provide a commit message"
    exit 1
fi

# Git operations
git add .
git commit -m "$1"
git push origin main

# Deploy to fly.io
echo "Deploying to fly.io..."

# Update secrets if needed
# fly secrets set NEW_SECRET="new-value"

# Deploy the application
fly deploy

# Monitor deployment
fly status

echo "Deployment completed!" 