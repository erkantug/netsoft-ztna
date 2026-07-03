#!/bin/bash
# Netsoft ZTNA - First Boot Detection Script
# Runs once on first boot, starts the wizard if not deployed

set -e

DEPLOY_FLAG="/opt/netsoft/.deployed"
FIRST_BOOT_FLAG="/opt/netsoft/.first-boot-done"
WIZARD_BIN="/usr/local/bin/netsoft-wizard"
WIZARD_SERVICE="netsoft-wizard.service"

log() {
    echo "[netsoft-first-boot] $(date -Iseconds) - $1"
    logger -t netsoft-first-boot "$1"
}

# Check if already deployed
if [ -f "$DEPLOY_FLAG" ]; then
    log "Already deployed. Nothing to do."
    touch "$FIRST_BOOT_FLAG"
    exit 0
fi

# Check if wizard binary exists
if [ ! -x "$WIZARD_BIN" ]; then
    log "ERROR: Wizard binary not found at $WIZARD_BIN"
    exit 1
fi

# Check if Docker is available
if ! command -v docker &>/dev/null; then
    log "ERROR: Docker not found"
    exit 1
fi

# Mark first boot as done
touch "$FIRST_BOOT_FLAG"

log "First boot detected. Starting wizard..."
systemctl start "$WIZARD_SERVICE" || {
    log "ERROR: Failed to start wizard service"
    exit 1
}

log "Wizard started on port 8080"
log "Open http://$(hostname -I | awk '{print $1}'):8080 to complete setup"

exit 0
