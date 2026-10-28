#!/usr/bin/env bash
# =============================================================================
# refresh-olm-install.sh — Force OLM to re-resolve from a refreshed catalog
# =============================================================================
#
# When an FBC catalog is rebuilt with updated bundle content (e.g. new
# relatedImages) but the bundle VERSION stays the same, OLM won't create
# a new InstallPlan.  This script evicts the stale CSV, InstallPlan, and
# Subscription, then recreates the Subscription so OLM resolves fresh
# from the current CatalogSource.
#
# Usage:
#   # Auto-detect the rhods/opendatahub Subscription
#   ./scripts/refresh-olm-install.sh
#
#   # Explicit
#   ./scripts/refresh-olm-install.sh --subscription rhods-operator \
#       --namespace redhat-ods-operator
#
#   # Source from another script (e.g. Shepard)
#   source scripts/refresh-olm-install.sh
#   refresh_olm_install --subscription rhods-operator --namespace redhat-ods-operator
#
# Safe on clusters where the CSV doesn't exist yet (exits cleanly).
# =============================================================================

set -euo pipefail

_log() { echo "[refresh-olm] $*"; }

refresh_olm_install() {
  local sub_name=""
  local sub_ns=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --subscription) sub_name="$2"; shift 2 ;;
      --namespace)    sub_ns="$2";   shift 2 ;;
      *) _log "Unknown arg: $1"; return 1 ;;
    esac
  done

  # Auto-detect if not specified
  if [[ -z "$sub_name" ]]; then
    local detected
    detected=$(kubectl get subscriptions.operators.coreos.com --all-namespaces \
      --no-headers 2>/dev/null \
      | awk '$2 ~ /^(rhods|opendatahub)-operator$/ {print $1, $2; exit}') || true
    if [[ -z "$detected" ]]; then
      _log "No rhods-operator or opendatahub-operator Subscription found — nothing to refresh"
      return 0
    fi
    sub_ns="${detected%% *}"
    sub_name="${detected##* }"
    _log "Auto-detected Subscription: $sub_name in $sub_ns"
  fi

  # ── 1. Read current Subscription spec ──────────────────────────────────
  local sub_json
  sub_json=$(kubectl get subscription "$sub_name" -n "$sub_ns" -o json 2>/dev/null) || {
    _log "Subscription $sub_name not found in $sub_ns — nothing to refresh"
    return 0
  }

  local csv_name
  csv_name=$(echo "$sub_json" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(d.get('status', {}).get('installedCSV', ''))
" 2>/dev/null) || true

  if [[ -z "$csv_name" ]]; then
    _log "No installedCSV on Subscription — fresh install, skipping"
    return 0
  fi

  _log "Installed CSV: $csv_name"

  # Save a clean Subscription spec (strip status, resourceVersion, uid, etc.)
  local sub_backup="/tmp/refresh-olm-sub-backup-$$.json"
  echo "$sub_json" | python3 -c "
import json, sys
d = json.load(sys.stdin)
clean = {
    'apiVersion': d['apiVersion'],
    'kind': d['kind'],
    'metadata': {
        'name': d['metadata']['name'],
        'namespace': d['metadata']['namespace'],
    },
    'spec': d['spec'],
}
# Remove fields that prevent re-creation
clean['spec'].pop('startingCSV', None)
json.dump(clean, sys.stdout, indent=2)
" > "$sub_backup" 2>/dev/null

  _log "Subscription spec saved to $sub_backup"

  # ── 2. Find all InstallPlans referencing this CSV ──────────────────────
  local ip_names
  ip_names=$(kubectl get installplan -n "$sub_ns" -o json 2>/dev/null \
    | python3 -c "
import json, sys
d = json.load(sys.stdin)
for ip in d.get('items', []):
    csvs = ip.get('spec', {}).get('clusterServiceVersionNames', [])
    if '$csv_name' in csvs:
        print(ip['metadata']['name'])
" 2>/dev/null) || true

  # ── 3. Find all namespaces where OLM copied the CSV ───────────────────
  local csv_namespaces
  csv_namespaces=$(kubectl get csv --all-namespaces --no-headers 2>/dev/null \
    | awk -v name="$csv_name" '$2 == name {print $1}') || true

  # ── 4. Delete: Subscription → InstallPlans → CSV (in order) ───────────
  _log "Deleting Subscription $sub_name ..."
  kubectl delete subscription "$sub_name" -n "$sub_ns" --ignore-not-found 2>/dev/null || true

  if [[ -n "$ip_names" ]]; then
    for ip in $ip_names; do
      _log "Deleting InstallPlan $ip ..."
      kubectl delete installplan "$ip" -n "$sub_ns" --ignore-not-found 2>/dev/null || true
    done
  fi

  for ns in $csv_namespaces; do
    _log "Deleting CSV $csv_name in $ns ..."
    kubectl delete csv "$csv_name" -n "$ns" --ignore-not-found 2>/dev/null || true
  done

  # Brief pause so OLM's cache clears
  sleep 5

  # ── 5. Recreate Subscription ──────────────────────────────────────────
  _log "Recreating Subscription from $sub_backup ..."
  kubectl apply -f "$sub_backup"

  # ── 6. Wait for CSV to come back ──────────────────────────────────────
  _log "Waiting for OLM to resolve and install $csv_name ..."
  local timeout=300
  local elapsed=0
  while [[ $elapsed -lt $timeout ]]; do
    local phase
    phase=$(kubectl get csv "$csv_name" -n "$sub_ns" \
      -o jsonpath='{.status.phase}' 2>/dev/null) || true
    if [[ "$phase" == "Succeeded" ]]; then
      _log "✅ CSV $csv_name re-installed successfully (${elapsed}s)"
      rm -f "$sub_backup"
      return 0
    fi
    local ip_count
    ip_count=$(kubectl get installplan -n "$sub_ns" --no-headers 2>/dev/null | wc -l) || true
    _log "  phase=${phase:-pending}  installplans=${ip_count}  (${elapsed}s/${timeout}s)"
    sleep 10
    elapsed=$((elapsed + 10))
  done
  _log "⚠️  CSV not Succeeded after ${timeout}s — check OLM logs"
  rm -f "$sub_backup"
  return 1
}

# Run if executed directly (not sourced)
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  refresh_olm_install "$@"
fi
