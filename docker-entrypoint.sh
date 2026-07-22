#!/bin/bash
set -e

# Ensure directory exists with correct ownership
mkdir -p /app/data
chown -R replyforge:replyforge /app/data

# Verify the setup worked (defense in depth)
if ! gosu replyforge bash -c "touch /app/data/.write_test && rm /app/data/.write_test"; then
    echo "ERROR: Cannot write to /app/data as replyforge user"
    echo "Directory permissions:"
    ls -la /app/data
    echo "Current user: $(id)"
    exit 1
fi

if [ "$1" = "hash-password" ]; then
    exec gosu replyforge /app/hash-password
fi

echo "Starting application as replyforge user..."
exec gosu replyforge /app/server "$@"