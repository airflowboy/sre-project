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
    # Phase D: random_password로 DB·캐시 비번 생성 → Secrets Manager에 저장
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
    # Phase D-2: ALB Controller IAM 정책 JSON을 업스트림 release에서 가져옴
    http = {
      source  = "hashicorp/http"
      version = "~> 3.0"
    }
  }
}
