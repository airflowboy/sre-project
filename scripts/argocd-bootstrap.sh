#!/usr/bin/env bash
# Ch10 Phase G - register the three ArgoCD Applications (ADR-021).
#
# Reads terraform outputs, fills the __PLACEHOLDER__ infra bindings in
# argocd/applications/*.yaml (IRSA ARNs, endpoints - values that only exist
# after `terraform apply`), and kubectl-applies them into the argocd ns.
#
# Run once, after ArgoCD is installed. From then on ArgoCD watches the helm
# charts in Git and syncs on its own - this script is the bootstrap, not the
# steady state.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TF="$ROOT/terraform/ch10"

echo "reading terraform outputs from $TF ..."
ISSUE_API_ROLE_ARN=$(terraform -chdir="$TF" output -raw issue_api_role_arn)
CONSUMER_ROLE_ARN=$(terraform -chdir="$TF" output -raw consumer_role_arn)
BOT_DETECTOR_ROLE_ARN=$(terraform -chdir="$TF" output -raw bot_detector_role_arn)
REDIS_ENDPOINT=$(terraform -chdir="$TF" output -raw redis_endpoint)
SECRET_DB_PASSWORD_ARN=$(terraform -chdir="$TF" output -raw secret_db_password_arn)
SECRET_DB_URL_ARN=$(terraform -chdir="$TF" output -raw secret_db_url_arn)
WAF_ACL_ARN=$(terraform -chdir="$TF" output -raw waf_acl_arn)
WAF_IPSET_NAME=$(terraform -chdir="$TF" output -raw waf_ipset_name)
WAF_IPSET_ID=$(terraform -chdir="$TF" output -raw waf_ipset_id)

# sed delimiter is '|' because ARNs contain '/' and ':' but never '|'.
for app in issue-api issuance-consumer bot-detector; do
  echo "applying Application/$app ..."
  sed -e "s|__ISSUE_API_ROLE_ARN__|${ISSUE_API_ROLE_ARN}|g" \
      -e "s|__CONSUMER_ROLE_ARN__|${CONSUMER_ROLE_ARN}|g" \
      -e "s|__BOT_DETECTOR_ROLE_ARN__|${BOT_DETECTOR_ROLE_ARN}|g" \
      -e "s|__REDIS_ENDPOINT__|${REDIS_ENDPOINT}|g" \
      -e "s|__SECRET_DB_PASSWORD_ARN__|${SECRET_DB_PASSWORD_ARN}|g" \
      -e "s|__SECRET_DB_URL_ARN__|${SECRET_DB_URL_ARN}|g" \
      -e "s|__WAF_ACL_ARN__|${WAF_ACL_ARN}|g" \
      -e "s|__WAF_IPSET_NAME__|${WAF_IPSET_NAME}|g" \
      -e "s|__WAF_IPSET_ID__|${WAF_IPSET_ID}|g" \
      "$ROOT/argocd/applications/$app.yaml" | kubectl apply -f -
done

echo
echo "done. watch sync state with:"
echo "  kubectl get applications -n argocd"
