#!/bin/sh
set -eu

mkdir -p /config /transcode
chown -R igloo:igloo /config /transcode
chmod 750 /config /transcode

exec gosu igloo:igloo "$@"
