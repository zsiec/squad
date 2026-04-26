#!/usr/bin/env bash
# Verifies the squad serve --install-service / --uninstall-service flow on linux.
# Designed to run inside a Docker container where systemctl is available
# (or stubbed) but not necessarily a live system systemd PID 1. The load-bearing
# checks are file-system effects (unit file, token, log dir); systemctl call
# failures from a non-init container are captured but tolerated.

set -uo pipefail

SQUAD_BIN="${SQUAD_BIN:-/usr/local/bin/squad}"
HOME_DIR="${HOME:-/root}"
UNIT_PATH="${HOME_DIR}/.config/systemd/user/squad-serve.service"
TOKEN_PATH="${HOME_DIR}/.squad/token"
LOG_DIR="${HOME_DIR}/.squad/logs"

echo "=== preconditions ==="
echo "uname: $(uname -a)"
echo "user: $(whoami)  uid=$(id -u)  home=$HOME_DIR"
echo "squad binary: $($SQUAD_BIN --help | head -1 || echo MISSING)"
echo "systemctl: $(command -v systemctl || echo MISSING)"
echo

echo "=== install ==="
$SQUAD_BIN serve --install-service
INSTALL_RC=$?
echo "install exit: $INSTALL_RC"
echo

echo "=== unit file ==="
if [ -f "$UNIT_PATH" ]; then
  ls -la "$UNIT_PATH"
  echo "---"
  cat "$UNIT_PATH"
else
  echo "UNIT FILE MISSING — DEFECT"
  exit 1
fi
echo

echo "=== token file ==="
if [ -f "$TOKEN_PATH" ]; then
  ls -la "$TOKEN_PATH"
  stat -c "mode=%a size=%s" "$TOKEN_PATH"
else
  echo "TOKEN FILE MISSING — DEFECT"
  exit 1
fi
echo

echo "=== log dir ==="
ls -la "$LOG_DIR" || echo "LOG DIR MISSING — DEFECT"
echo

echo "=== status ==="
$SQUAD_BIN serve --service-status
STATUS_RC=$?
echo "status exit: $STATUS_RC"
echo

echo "=== uninstall ==="
$SQUAD_BIN serve --uninstall-service
UNINSTALL_RC=$?
echo "uninstall exit: $UNINSTALL_RC"
echo

echo "=== post-uninstall: unit file should be gone ==="
if [ -f "$UNIT_PATH" ]; then
  echo "UNIT FILE STILL PRESENT — DEFECT"
  cat "$UNIT_PATH"
  exit 1
else
  echo "unit file removed ✓"
fi
echo

echo "=== post-uninstall: token preserved (per design) ==="
if [ -f "$TOKEN_PATH" ]; then
  echo "token preserved ✓"
else
  echo "token absent (would be a regression — uninstall shouldn't remove it)"
fi
echo

echo "=== verify-linux-install.sh complete ==="
