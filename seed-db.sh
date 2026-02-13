#!/bin/bash
set -e

echo "Waiting for verifier server to complete migrations..."
sleep 10

echo "Seeding verifier database..."
psql -v ON_ERROR_STOP=1 <<-EOSQL
    \connect vultisig-verifier
    \i /seeds/00_default_agent.sql
    \i /seeds/01_plugins.sql
    \i /seeds/04_tags.sql
    \i /seeds/06_plugin_apikey.sql
    \i /seeds/07_merkle_apikey.sql
EOSQL

echo "Database seeding completed!"