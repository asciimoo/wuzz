#!/bin/sh
set -e

if [ "${1#-}" != "${1}" ] || [ -z "$(command -v "${1}")" ]; then
  sleep 0.01
  set -- wuzz "$@"
fi

exec "$@"