#!/bin/bash

CHART_URL=$1
CHART_VERSION=$2

if [[ -z "$CHART_URL" ]]; then
  echo -n "Chart url: "
  read -r CHART_URL
fi

if [[ -z "$CHART_VERSION" ]]; then
  echo -n "Chart version: "
  read -r CHART_VERSION
fi


echo "Rollout to local cluster"

# Post using curl

curl -X POST -H "Content-Type: application/json" \
    -d "{\"chart\": \"$CHART_URL\", \"version\": \"$CHART_VERSION\"}" \
    http://localhost:8080/github/rollout
