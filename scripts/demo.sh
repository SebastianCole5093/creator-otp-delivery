#!/bin/sh
set -eu

: "${PHONE:?set PHONE in E.164 format}"
: "${CODE:?set CODE to the SMS code}"

start=$(curl --fail-with-body --silent --show-error \
  --request POST http://localhost:8080/login/code \
  --header 'Content-Type: application/json' \
  --data "{\"phone\":\"$PHONE\",\"creator_id\":\"creator-7\",\"asset_id\":\"asset-9\"}")

challenge_id=$(printf '%s' "$start" | sed -n 's/.*"challenge_id":"\([^"]*\)".*/\1/p')
test -n "$challenge_id"

curl --fail-with-body --silent --show-error \
  --request POST http://localhost:8080/login/verify \
  --header 'Content-Type: application/json' \
  --data "{\"challenge_id\":\"$challenge_id\",\"code\":\"$CODE\"}"
