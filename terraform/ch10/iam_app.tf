# Ch 10 Phase D — IRSA role for the issue-api app (ADR-014).
#
# This role is assumed by the K8s ServiceAccount `default/issue-api` (set via
# variables; the SA itself is created in Phase D-2 as part of the Helm chart).
# The trust policy hardcodes that exact SA so no other Pod can use this role.
#
# Permissions: read the two Phase D Secrets Manager secrets — nothing else.

# Strip the "https://" from the OIDC issuer URL because IAM conditions
# expect the bare host:path form (e.g., oidc.eks.ap-northeast-2.amazonaws.com/id/XXX).
locals {
  oidc_provider_url = replace(aws_iam_openid_connect_provider.eks.url, "https://", "")
}

data "aws_iam_policy_document" "issue_api_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.eks.arn]
    }

    # Pin to one specific SA in one specific namespace.
    condition {
      test     = "StringEquals"
      variable = "${local.oidc_provider_url}:sub"
      values   = ["system:serviceaccount:${var.app_namespace}:${var.app_service_account}"]
    }

    # Audience must match what the SA token is issued for.
    condition {
      test     = "StringEquals"
      variable = "${local.oidc_provider_url}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "issue_api" {
  name               = "${var.cluster_name}-issue-api"
  assume_role_policy = data.aws_iam_policy_document.issue_api_assume.json
  description        = "IRSA role for issue-api Pod - read 2 secrets only"
}

# Read-only on the two specific secret ARNs. No wildcard.
data "aws_iam_policy_document" "issue_api_secrets" {
  statement {
    effect  = "Allow"
    actions = ["secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"]
    resources = [
      aws_secretsmanager_secret.db_password.arn,
      aws_secretsmanager_secret.db_url.arn,
    ]
  }
}

resource "aws_iam_policy" "issue_api_secrets" {
  name   = "${var.cluster_name}-issue-api-secrets"
  policy = data.aws_iam_policy_document.issue_api_secrets.json
}

resource "aws_iam_role_policy_attachment" "issue_api_secrets" {
  role       = aws_iam_role.issue_api.name
  policy_arn = aws_iam_policy.issue_api_secrets.arn
}
