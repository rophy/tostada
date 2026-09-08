#!/bin/bash
set -e

RESULTS_DIR=/results
mkdir -p "$RESULTS_DIR"

echo "=== Waiting for gateway to be ready ==="
until curl -sf "$TOSTADA_E2E_URL/api/auth/login" > /dev/null 2>&1; do
  echo "  waiting..."
  sleep 2
done
echo "Gateway is ready."

echo ""
echo "=== Running Go API e2e tests ==="
e2e-api -test.v -test.count=1 2>&1 | tee "$RESULTS_DIR/api-tests.log"
API_EXIT=${PIPESTATUS[0]}

echo ""
echo "=== Running Playwright e2e tests ==="
cd /e2e/web
npx playwright test 2>&1 | tee "$RESULTS_DIR/playwright.log"
PW_EXIT=${PIPESTATUS[0]}

echo ""
echo "=== Flushing server coverage ==="
if curl -sf -X POST http://tostada:8080/debug/coverage/flush -o "$RESULTS_DIR/coverage.tar"; then
  echo "Coverage saved to $RESULTS_DIR/coverage.tar"
else
  echo "Coverage flush failed (server may not have coverage instrumentation)"
fi

echo ""
echo "=== Results ==="
echo "API tests: exit $API_EXIT"
echo "Playwright tests: exit $PW_EXIT"

if [ "$API_EXIT" -ne 0 ] || [ "$PW_EXIT" -ne 0 ]; then
  exit 1
fi
