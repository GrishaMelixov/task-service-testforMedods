#!/usr/bin/env bash
# seed.sh — creates one schedule of each kind and a few one-off tasks.
# Usage: bash scripts/seed.sh [BASE_URL]
# Default BASE_URL: http://localhost:8080

set -euo pipefail

BASE="${1:-http://localhost:8080}"
API="$BASE/api/v1"

GREEN='\033[0;32m'
CYAN='\033[0;36m'
RESET='\033[0m'

post() {
  local label="$1"
  local path="$2"
  local body="$3"
  echo -e "${CYAN}>>> $label${RESET}"
  curl -sf -X POST "$API$path" \
    -H 'Content-Type: application/json' \
    -d "$body" | jq .
  echo
}

get() {
  local label="$1"
  local path="$2"
  echo -e "${CYAN}>>> $label${RESET}"
  curl -sf "$API$path" | jq .
  echo
}

echo -e "${GREEN}=== Seeding schedules ===${RESET}"
echo

# 1. daily_every_n — fires every day
post "daily_every_n: daily patient round" /schedules '{
  "title": "Daily patient round",
  "description": "Morning round of all wards",
  "default_status": "new",
  "kind": "daily_every_n",
  "params": {"n": 1},
  "start_date": "2026-04-01",
  "timezone": "Europe/Moscow"
}'

# 2. daily_every_n — fires every 3 days
post "daily_every_n: physiotherapy (every 3 days)" /schedules '{
  "title": "Physiotherapy session",
  "description": "Session for ward 3 patients",
  "default_status": "new",
  "kind": "daily_every_n",
  "params": {"n": 3},
  "start_date": "2026-04-01",
  "timezone": "Europe/Moscow"
}'

# 3. monthly_days — 1st and 15th of every month
post "monthly_days: medication audit" /schedules '{
  "title": "Monthly medication audit",
  "description": "Count controlled substances",
  "default_status": "new",
  "kind": "monthly_days",
  "params": {"days": [1, 15]},
  "start_date": "2026-04-01",
  "timezone": "Europe/Moscow"
}'

# 4. specific_dates — ad-hoc inspection dates
post "specific_dates: equipment inspection" /schedules '{
  "title": "Equipment safety inspection",
  "description": "Annual inspection per regulatory requirement",
  "default_status": "new",
  "kind": "specific_dates",
  "params": {"dates": ["2026-05-01", "2026-09-01", "2027-01-15"]},
  "start_date": "2026-05-01",
  "timezone": "Europe/Moscow"
}'

# 5. even_odd — even days only
post "even_odd: dialysis (even days)" /schedules '{
  "title": "Dialysis procedure",
  "description": "Dialysis for patients in ward 5",
  "default_status": "new",
  "kind": "even_odd",
  "params": {"parity": "even"},
  "start_date": "2026-04-02",
  "timezone": "Europe/Moscow"
}'

# 6. even_odd — odd days only
post "even_odd: physiotherapy (odd days)" /schedules '{
  "title": "Physiotherapy (alternate group)",
  "description": "Group B physiotherapy sessions",
  "default_status": "new",
  "kind": "even_odd",
  "params": {"parity": "odd"},
  "start_date": "2026-04-01",
  "timezone": "Europe/Moscow"
}'

echo -e "${GREEN}=== Seeding one-off tasks ===${RESET}"
echo

post "one-off task: urgent consult" /tasks '{
  "title": "Urgent cardiology consult",
  "description": "Patient in bed 12 needs immediate review",
  "status": "new"
}'

post "one-off task: supply order" /tasks '{
  "title": "Order surgical supplies",
  "status": "new"
}'

echo -e "${GREEN}=== Listing all schedules ===${RESET}"
echo
get "GET /api/v1/schedules" /schedules

echo -e "${GREEN}=== Listing all tasks (with scheduler filter example) ===${RESET}"
echo
get "GET /api/v1/tasks?schedule_id=1" "/tasks?schedule_id=1"

echo
echo -e "${GREEN}Done. The generator will materialise upcoming tasks on its next cycle."
echo -e "Set GENERATOR_INTERVAL=5s to see results quickly.${RESET}"
