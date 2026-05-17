# Ch 10 Phase D-2 - GitHub Actions OIDC -> AWS (ADR-015).
#
# GitHub Actions presents a signed OIDC token; AWS STS validates against this
# provider, then issues short-lived credentials. No access keys live anywhere.
#
# The IAM role's trust policy locks to a SPECIFIC repo and branch via the
# `sub` claim, so a forked PR or another repo cannot assume it.

# GitHub's well-known OIDC issuer.
resource "aws_iam_openid_connect_provider" "github" {
  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  # GitHub publishes a SHA-1 thumbprint; AWS now validates the cert chain
  # internally so the value here is documentation more than a security control.
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1"]
}

data "aws_iam_policy_document" "github_actions_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.github.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    # Pin to one repo + main branch. Fork PRs do NOT match this sub pattern.
    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values   = ["repo:${var.github_owner}/${var.github_repo}:ref:refs/heads/main"]
    }
  }
}

resource "aws_iam_role" "github_actions" {
  name               = "${var.cluster_name}-github-actions"
  assume_role_policy = data.aws_iam_policy_document.github_actions_assume.json
  description        = "Assumed by GitHub Actions via OIDC to push images to ECR"
}

# ECR push policy - GetAuthorizationToken is account-wide (resource must be *),
# the layer/manifest actions are scoped to our one repo ARN.
data "aws_iam_policy_document" "ecr_push" {
  statement {
    sid       = "GetAuthToken"
    effect    = "Allow"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }
  statement {
    sid    = "PushPull"
    effect = "Allow"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:BatchGetImage",
      "ecr:CompleteLayerUpload",
      "ecr:GetDownloadUrlForLayer",
      "ecr:InitiateLayerUpload",
      "ecr:PutImage",
      "ecr:UploadLayerPart",
    ]
    resources = [aws_ecr_repository.issue_api.arn]
  }
}

resource "aws_iam_policy" "ecr_push" {
  name   = "${var.cluster_name}-ecr-push"
  policy = data.aws_iam_policy_document.ecr_push.json
}

resource "aws_iam_role_policy_attachment" "github_actions_ecr" {
  role       = aws_iam_role.github_actions.name
  policy_arn = aws_iam_policy.ecr_push.arn
}
