# Ch 10 Phase E-1 - IRSA role for the issuance-consumer Pod (ADR-014 pattern).
#
# Permissions: read both Phase D-1 secrets - the consumer fetches db/url at
# startup; db/password is included so a future rotation flow can pull a fresh
# value without redeploying.

data "aws_iam_policy_document" "consumer_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.eks.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_provider_url}:sub"
      values   = ["system:serviceaccount:${var.app_namespace}:issuance-consumer"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_provider_url}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "consumer" {
  name               = "${var.cluster_name}-issuance-consumer"
  assume_role_policy = data.aws_iam_policy_document.consumer_assume.json
  description        = "IRSA role for issuance-consumer Pod - read 2 secrets only"
}

data "aws_iam_policy_document" "consumer_secrets" {
  statement {
    effect  = "Allow"
    actions = ["secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"]
    resources = [
      aws_secretsmanager_secret.db_password.arn,
      aws_secretsmanager_secret.db_url.arn,
    ]
  }
}

resource "aws_iam_policy" "consumer_secrets" {
  name   = "${var.cluster_name}-consumer-secrets"
  policy = data.aws_iam_policy_document.consumer_secrets.json
}

resource "aws_iam_role_policy_attachment" "consumer_secrets" {
  role       = aws_iam_role.consumer.name
  policy_arn = aws_iam_policy.consumer_secrets.arn
}
