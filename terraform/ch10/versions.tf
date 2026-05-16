terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    # Phase B: tls 데이터 소스로 EKS OIDC issuer의 thumbprint 자동 추출
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }
}
