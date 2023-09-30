#!/bin/bash
set -e

# These speed up the DB for dev time by turning off all safety aspects.
# You would be insane to do this in production.

echo "Disabling all safety features of DB (for DEV TIME ONLY)"

echo "wal_level = minimal" >> "$PGDATA/postgresql.conf"
echo "max_wal_senders = 0" >> "$PGDATA/postgresql.conf"
echo "fsync = off" >> "$PGDATA/postgresql.conf"
echo "synchronous_commit = off" >> "$PGDATA/postgresql.conf"
echo "full_page_writes = off" >> "$PGDATA/postgresql.conf"