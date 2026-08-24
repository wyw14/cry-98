#!/bin/sh
set -eu
docker build -f benzhi.Dockerfile -t cryosafe:local .
