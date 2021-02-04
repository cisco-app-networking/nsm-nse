#!/bin/bash

set -e

echo "Build NSE images"
ORG=appn TAG=${BRANCH_NAME} make docker-vl3
