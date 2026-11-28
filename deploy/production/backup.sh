#!/bin/sh
# Nightly backups for the production stack (compose.yml, service "backup").
# Takes a compressed custom-format dump now, then once a day, and deletes
# dumps older than BACKUP_KEEP_DAYS. Restore with `task prod:restore -- <file>`.
set -eu

keep="${BACKUP_KEEP_DAYS:-14}"

dump() {
	name="fuxi-$(date -u +%Y%m%d-%H%M%S).dump"
	# Write to a temporary name first, so a failed dump never looks like a good one.
	if pg_dump --format=custom --no-owner --file="/backups/.$name.partial"; then
		mv "/backups/.$name.partial" "/backups/$name"
		echo "backup: wrote $name ($(du -h "/backups/$name" | cut -f1))"
	else
		rm -f "/backups/.$name.partial"
		echo "backup: pg_dump failed" >&2
	fi
	find /backups -maxdepth 1 -name 'fuxi-*.dump' -mtime +"$keep" -print -delete | sed 's/^/backup: removed /'
}

# `sh /backup.sh once` takes one dump and exits (task prod:backup).
if [ "${1:-}" = once ]; then
	dump
	exit 0
fi

while true; do
	dump
	sleep 86400
done
