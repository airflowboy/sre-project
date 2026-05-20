# Ch 10 Phase F-1 - AWS WAF v2 in front of the ALB (ADR-018).
#
# REGIONAL scope is the right one for ALB/API Gateway/Cognito; CLOUDFRONT is
# a separate scope for CloudFront distributions.
#
# We don't associate the ACL to the ALB here - the ALB is created by the
# AWS Load Balancer Controller from a Kubernetes Ingress, so the Helm chart
# carries the wafv2-acl-arn annotation and the controller does the
# AssociateWebACL call for us.

# Phase F-2 - dynamic bot blocklist (ADR-020).
# Terraform creates the IPSet empty; the bot-detector Pod owns its contents
# via UpdateIPSet at runtime, so we ignore_changes on addresses.
resource "aws_wafv2_ip_set" "bot_blocklist" {
  name               = "${var.cluster_name}-bot-blocklist"
  description        = "IPs flagged by the bot-detector (Phase F-2)"
  scope              = "REGIONAL"
  ip_address_version = "IPV4"
  addresses          = []

  lifecycle {
    ignore_changes = [addresses]
  }

  tags = { Name = "${var.cluster_name}-bot-blocklist" }
}

resource "aws_wafv2_web_acl" "main" {
  name        = "${var.cluster_name}-waf"
  description = "Issue-API ALB protection - managed rules + rate-based + bot blocklist"
  scope       = "REGIONAL"

  default_action {
    allow {}
  }

  # ---- Phase F-2: bot blocklist - priority 0, evaluated before everything ----
  rule {
    name     = "BotBlocklist"
    priority = 0
    action {
      block {}
    }
    statement {
      ip_set_reference_statement {
        arn = aws_wafv2_ip_set.bot_blocklist.arn
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "BotBlocklist"
      sampled_requests_enabled   = true
    }
  }

  # ---- AWS Managed: OWASP top-10 baseline (CommonRuleSet) ----
  rule {
    name     = "AWSManagedRulesCommonRuleSet"
    priority = 1
    override_action {
      none {}
    }
    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesCommonRuleSet"
        vendor_name = "AWS"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "CommonRuleSet"
      sampled_requests_enabled   = true
    }
  }

  # ---- AWS Managed: known bad payloads (path traversal, log4shell, etc.) ----
  rule {
    name     = "AWSManagedRulesKnownBadInputsRuleSet"
    priority = 2
    override_action {
      none {}
    }
    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesKnownBadInputsRuleSet"
        vendor_name = "AWS"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "KnownBadInputs"
      sampled_requests_enabled   = true
    }
  }

  # ---- AWS Managed: IP reputation list maintained by Amazon ----
  rule {
    name     = "AWSManagedRulesAmazonIpReputationList"
    priority = 3
    override_action {
      none {}
    }
    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesAmazonIpReputationList"
        vendor_name = "AWS"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "IpReputation"
      sampled_requests_enabled   = true
    }
  }

  # ---- Custom: rate-based limit per IP ----
  # 100 requests per 5-minute sliding window per source IP. Aggressive on
  # purpose for the smoke test - production would start much higher and tune
  # down using CloudWatch metrics.
  rule {
    name     = "RateLimitPerIP"
    priority = 10
    action {
      block {}
    }
    statement {
      rate_based_statement {
        limit              = 100
        aggregate_key_type = "IP"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "RateLimitPerIP"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${var.cluster_name}-waf"
    sampled_requests_enabled   = true
  }

  tags = { Name = "${var.cluster_name}-waf" }
}
