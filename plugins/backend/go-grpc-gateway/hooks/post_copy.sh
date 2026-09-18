#!/bin/sh
# Runs after all plugins' files are copied and merged, from the generated
# project's root (see plugins/README.md — "Hooks"). KILN_APP_NAME is one of
# the values every kiln new run resolves before generation.
set -eu

DIR="${KILN_APP_NAME}-api"
if [ ! -d "$DIR" ]; then
	echo "warning: $DIR not found, skipping go mod tidy" >&2
	exit 0
fi

echo "==> Tidying Go modules in $DIR"
(cd "$DIR" && go mod tidy)
