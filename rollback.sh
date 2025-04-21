#!/bin/bash

# Get the previous deployment
PREVIOUS_VERSION=$(fly releases -a unicorn-app-backend | grep -A 1 "v" | tail -n 1 | awk '{print $1}')

# Rollback to previous version
fly deploy --image registry.fly.io/unicorn-app-backend:$PREVIOUS_VERSION

echo "Rolled back to version $PREVIOUS_VERSION" 