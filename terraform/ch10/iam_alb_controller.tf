# Ch 10 Phase D-2 - IRSA role for AWS Load Balancer Controller (ADR-013).
#
# The controller runs as a Deployment in kube-system with the
# `aws-load-balancer-controller` ServiceAccount. It watches Ingress resources
# and provisions/destroys real ALBs in the AWS account. To do that it needs
# a wide set of EC2/ELB/IAM permissions - we use the upstream policy JSON
# verbatim (fetched at apply time from the project's GitHub release).

locals {
  # Same OIDC URL helper as iam_app.tf - the EKS cluster OIDC provider,
  # not to be confused with the GitHub OIDC provider in github_oidc.tf.
  alb_oidc_url = replace(aws_iam_openid_connect_provider.eks.url, "https://", "")
}

# Upstream-maintained policy. Pinning to a release tag means our apply
# is reproducible; switching versions is a one-line ref change.
data "http" "alb_controller_policy" {
  url = "https://raw.githubusercontent.com/kubernetes-sigs/aws-load-balancer-controller/v2.8.2/docs/install/iam_policy.json"
}

resource "aws_iam_policy" "alb_controller" {
  name        = "${var.cluster_name}-alb-controller"
  description = "AWS Load Balancer Controller managed policy (upstream v2.8.2)"
  policy      = data.http.alb_controller_policy.response_body
}

data "aws_iam_policy_document" "alb_controller_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.eks.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.alb_oidc_url}:sub"
      values   = ["system:serviceaccount:kube-system:aws-load-balancer-controller"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.alb_oidc_url}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "alb_controller" {
  name               = "${var.cluster_name}-alb-controller"
  assume_role_policy = data.aws_iam_policy_document.alb_controller_assume.json
  description        = "IRSA role for aws-load-balancer-controller SA in kube-system"
}

resource "aws_iam_role_policy_attachment" "alb_controller" {
  role       = aws_iam_role.alb_controller.name
  policy_arn = aws_iam_policy.alb_controller.arn
}
