# Ch 10 Phase F-2 - IRSA role for the bot-detector Pod (ADR-019/020).
#
# Permissions: read + update ONE WAF IPSet. Nothing else. If the Pod is
# compromised the blast radius is that single blocklist - it cannot touch
# the Web ACL, managed rules, or any other IPSet.

data "aws_iam_policy_document" "bot_detector_assume" {
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
      values   = ["system:serviceaccount:${var.app_namespace}:bot-detector"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_provider_url}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "bot_detector" {
  name               = "${var.cluster_name}-bot-detector"
  assume_role_policy = data.aws_iam_policy_document.bot_detector_assume.json
  description        = "IRSA role for bot-detector Pod - update one WAF IPSet only"
}

# Get + Update on exactly the bot-blocklist IPSet ARN. No wildcard.
data "aws_iam_policy_document" "bot_detector_waf" {
  statement {
    effect    = "Allow"
    actions   = ["wafv2:GetIPSet", "wafv2:UpdateIPSet"]
    resources = [aws_wafv2_ip_set.bot_blocklist.arn]
  }
}

resource "aws_iam_policy" "bot_detector_waf" {
  name   = "${var.cluster_name}-bot-detector-waf"
  policy = data.aws_iam_policy_document.bot_detector_waf.json
}

resource "aws_iam_role_policy_attachment" "bot_detector_waf" {
  role       = aws_iam_role.bot_detector.name
  policy_arn = aws_iam_policy.bot_detector_waf.arn
}
