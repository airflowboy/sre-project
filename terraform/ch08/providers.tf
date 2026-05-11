provider "aws" {
  region = var.region

  # Every resource created by this config gets these tags automatically —
  # makes "what did Terraform create / what is this for" obvious in the console.
  default_tags {
    tags = {
      Project   = "sre-roadmap"
      Chapter   = "08"
      ManagedBy = "terraform"
    }
  }
}
