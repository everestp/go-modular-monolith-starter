#!/usr/bin/env bash
set -euo pipefail

base_url="${BASE_URL:-http://localhost:8080}"
suffix="$(date +%s%N)"
password='strong-password-123'
email="e2e-${suffix}@example.com"

register=$(curl --fail --silent --show-error \
	-H 'Content-Type: application/json' \
	-d "{\"email\":\"${email}\",\"password\":\"${password}\"}" \
	"${base_url}/api/v1/auth/register")
refresh=$(jq -er '.refresh_token' <<<"${register}")
login=$(curl --fail --silent --show-error \
	-H 'Content-Type: application/json' \
	-d "{\"email\":\"${email}\",\"password\":\"${password}\"}" \
	"${base_url}/api/v1/auth/login")
access=$(jq -er '.access_token' <<<"${login}")

curl --fail --silent --show-error -H "Authorization: Bearer ${access}" \
	"${base_url}/api/v1/auth/me" | jq -e --arg email "${email}" '.email == $email' >/dev/null

created=$(curl --fail --silent --show-error \
	-H 'Content-Type: application/json' \
	-H "Authorization: Bearer ${access}" \
	-d '{"title":"e2e note","content":"original"}' \
	"${base_url}/api/v1/notes")
note_id=$(jq -er '.id' <<<"${created}")
test "$(jq -er '.version' <<<"${created}")" = 1

curl --fail --silent --show-error -H "Authorization: Bearer ${access}" \
	"${base_url}/api/v1/notes/${note_id}" | jq -e '.title == "e2e note"' >/dev/null
curl --fail --silent --show-error -H "Authorization: Bearer ${access}" \
	"${base_url}/api/v1/notes?limit=20" | jq -e --arg id "${note_id}" 'any(.items[]; .id == $id)' >/dev/null

curl --fail --silent --show-error -X PUT \
	-H 'Content-Type: application/json' \
	-H "Authorization: Bearer ${access}" \
	-H 'If-Match: "1"' \
	-d '{"title":"replaced","content":"updated"}' \
	"${base_url}/api/v1/notes/${note_id}" | jq -e '.version == 2' >/dev/null
curl --fail --silent --show-error -X PATCH \
	-H 'Content-Type: application/json' \
	-H "Authorization: Bearer ${access}" \
	-H 'If-Match: "2"' \
	-d '{"content":"patched"}' \
	"${base_url}/api/v1/notes/${note_id}" | jq -e '.version == 3 and .content == "patched"' >/dev/null

other_email="e2e-other-${suffix}@example.com"
other=$(curl --fail --silent --show-error \
	-H 'Content-Type: application/json' \
	-d "{\"email\":\"${other_email}\",\"password\":\"${password}\"}" \
	"${base_url}/api/v1/auth/register")
other_access=$(jq -er '.access_token' <<<"${other}")
other_status=$(curl --silent --show-error -o /dev/null -w '%{http_code}' \
	-H "Authorization: Bearer ${other_access}" \
	"${base_url}/api/v1/notes/${note_id}")
test "${other_status}" = 404

delete_status=$(curl --silent --show-error -o /dev/null -w '%{http_code}' -X DELETE \
	-H "Authorization: Bearer ${access}" \
	-H 'If-Match: "3"' \
	"${base_url}/api/v1/notes/${note_id}")
test "${delete_status}" = 204

rotated=$(curl --fail --silent --show-error \
	-H 'Content-Type: application/json' \
	-d "{\"refresh_token\":\"${refresh}\"}" \
	"${base_url}/api/v1/auth/refresh")
rotated_refresh=$(jq -er '.refresh_token' <<<"${rotated}")
replay_status=$(curl --silent --show-error -o /dev/null -w '%{http_code}' \
	-H 'Content-Type: application/json' \
	-d "{\"refresh_token\":\"${refresh}\"}" \
	"${base_url}/api/v1/auth/refresh")
test "${replay_status}" = 401
family_status=$(curl --silent --show-error -o /dev/null -w '%{http_code}' \
	-H 'Content-Type: application/json' \
	-d "{\"refresh_token\":\"${rotated_refresh}\"}" \
	"${base_url}/api/v1/auth/refresh")
test "${family_status}" = 401

printf 'E2E passed: auth, note CRUD, owner isolation, refresh rotation/reuse detection.\n'
