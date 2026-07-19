#!/bin/bash
################################################################################
# Test a MaaS PR against latest ODH + stable ai-gateway-operator
#
# Deploys the ODH operator (main-branch "latest" catalog), pins ai-gateway-operator
# to its stable image, pins maas-controller/maas-api to PR-built images, and runs
# the full MaaS e2e suite against that combination. This is the integration gate
# for main → stable promotion PRs: it proves a PR's images still work against the
# stable AIGateway component and the latest ODH operator build.
#
# Requires an OpenShift cluster (oc logged in with cluster-admin). This drives
# deploy.sh in --deployment-mode operator, which is the only path that exercises
# the ODH operator's own ModelsAsService + AIGateway component reconcilers (the
# default CI path, --deployment-mode kustomize, deploys maas-controller/maas-api
# directly and never touches ai-gateway-operator at all).
#
# USAGE:
#   ./scripts/test-pr-against-odh-latest.sh \
#     --maas-controller-image quay.io/opendatahub/maas-controller:pr-406 \
#     --maas-api-image quay.io/opendatahub/maas-api:pr-232
#
# OPTIONS:
#   --maas-controller-image <image>     PR-built maas-controller image
#   --maas-api-image <image>            PR-built maas-api image
#   --ai-gateway-operator-image <image> ai-gateway-operator image
#                                        (default: quay.io/opendatahub/ai-gateway-operator:stable)
#   --odh-catalog <image>                ODH catalog image
#                                        (default: quay.io/opendatahub/opendatahub-operator-catalog:latest)
#   --skip-e2e                           Deploy only; skip running the pytest e2e suite
#   --help                               Show this help message
#
# At least one of --maas-controller-image / --maas-api-image is required — otherwise
# this is just a normal ODH-latest + ai-gateway-operator-stable deploy with no PR under test.
#
# Any other environment variables understood by test/e2e/scripts/prow_run_smoke_test.sh
# (SKIP_DEPLOYMENT, SKIP_VALIDATION, INSECURE_HTTP, EXTERNAL_OIDC, etc.) are passed through
# unchanged; see that script's header for the full list.
################################################################################

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

DEFAULT_ODH_CATALOG="quay.io/opendatahub/opendatahub-operator-catalog:latest"
DEFAULT_AI_GATEWAY_OPERATOR_IMAGE="quay.io/opendatahub/ai-gateway-operator:stable"

MAAS_CONTROLLER_IMAGE="${MAAS_CONTROLLER_IMAGE:-}"
MAAS_API_IMAGE="${MAAS_API_IMAGE:-}"
AI_GATEWAY_OPERATOR_IMAGE="${AI_GATEWAY_OPERATOR_IMAGE:-$DEFAULT_AI_GATEWAY_OPERATOR_IMAGE}"
OPERATOR_CATALOG="${OPERATOR_CATALOG:-$DEFAULT_ODH_CATALOG}"
SKIP_E2E="false"

show_help() {
  sed -n '2,/^####*$/p' "${BASH_SOURCE[0]}" | sed '1d;$d;s/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --maas-controller-image)
      MAAS_CONTROLLER_IMAGE="${2:?--maas-controller-image requires a value}"
      shift 2
      ;;
    --maas-api-image)
      MAAS_API_IMAGE="${2:?--maas-api-image requires a value}"
      shift 2
      ;;
    --ai-gateway-operator-image)
      AI_GATEWAY_OPERATOR_IMAGE="${2:?--ai-gateway-operator-image requires a value}"
      shift 2
      ;;
    --odh-catalog)
      OPERATOR_CATALOG="${2:?--odh-catalog requires a value}"
      shift 2
      ;;
    --skip-e2e)
      SKIP_E2E="true"
      shift
      ;;
    --help|-h)
      show_help
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Use --help for usage information" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$MAAS_CONTROLLER_IMAGE" && -z "$MAAS_API_IMAGE" ]]; then
  echo "ERROR: at least one of --maas-controller-image / --maas-api-image is required." >&2
  echo "       (otherwise there is no PR image under test — this would just deploy ODH" >&2
  echo "       latest + ai-gateway-operator stable with default MaaS images)" >&2
  exit 1
fi

echo "==================================================="
echo "  Testing MaaS PR against ODH latest"
echo "==================================================="
echo "  ODH catalog:            ${OPERATOR_CATALOG}"
echo "  ai-gateway-operator:    ${AI_GATEWAY_OPERATOR_IMAGE}"
echo "  maas-controller image:  ${MAAS_CONTROLLER_IMAGE:-<default>}"
echo "  maas-api image:         ${MAAS_API_IMAGE:-<default>}"
echo "  Skip e2e:               ${SKIP_E2E}"
echo "==================================================="
echo ""

export OPERATOR_CATALOG
export AI_GATEWAY_OPERATOR_IMAGE
export MAAS_CONTROLLER_IMAGE
export MAAS_API_IMAGE
export DEPLOY_MODE="operator"
export SKIP_DEPLOYMENT="${SKIP_DEPLOYMENT:-false}"
export SKIP_VALIDATION="${SKIP_E2E}"

if [[ "$SKIP_E2E" == "true" ]]; then
  # prow_run_smoke_test.sh always runs pytest as its final phase; there's no flag to skip
  # just that part, so drive the deploy phases directly via deploy.sh instead.
  echo "Deploying only (--skip-e2e); running deploy.sh directly instead of the full e2e wrapper..."
  deploy_cmd=(
    "$PROJECT_ROOT/scripts/deploy.sh"
    --deployment-mode operator
    --operator-type odh
    --operator-catalog "$OPERATOR_CATALOG"
    --ai-gateway-operator-image "$AI_GATEWAY_OPERATOR_IMAGE"
  )
  [[ -n "$MAAS_CONTROLLER_IMAGE" ]] && deploy_cmd+=(--maas-controller-image "$MAAS_CONTROLLER_IMAGE")
  [[ -n "$MAAS_API_IMAGE" ]] && deploy_cmd+=(--maas-api-image "$MAAS_API_IMAGE")
  exec "${deploy_cmd[@]}"
fi

exec "$PROJECT_ROOT/test/e2e/scripts/prow_run_smoke_test.sh"
