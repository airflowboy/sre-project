provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project   = "sre-roadmap"
      Chapter   = "10"
      ManagedBy = "terraform"
    }
  }
}
