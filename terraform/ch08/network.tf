# Phase B — a VPC built from scratch (we do NOT use the account's default VPC).
#
# Layout:
#   aws_vpc                       10.0.0.0/16
#     └─ aws_subnet (public)      10.0.1.0/24 in ap-northeast-2a, auto public IP
#   aws_internet_gateway          attached to the VPC (the door to the internet)
#   aws_route_table (public)      0.0.0.0/0 -> IGW
#     └─ aws_route_table_association   binds the public subnet to that route table
#   aws_security_group (web)      inbound: SSH from my IP, HTTP from anywhere; outbound: all
#
# "Public subnet" = 4 things together: a subnet + an IGW + a route to it + public IPs.
# Miss any one and instances there can't reach the internet.

resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true # instances get public DNS names
  tags                 = { Name = "${var.name_prefix}-vpc" }
}

resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = var.public_subnet_cidr
  availability_zone       = var.az
  map_public_ip_on_launch = true
  tags                    = { Name = "${var.name_prefix}-public-a" }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
  tags   = { Name = "${var.name_prefix}-igw" }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = { Name = "${var.name_prefix}-public-rt" }
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

# Detect the public IP of whatever is running terraform (rocky-master -> home router),
# so the SSH rule below is scoped to just us instead of the whole internet.
data "http" "myip" {
  url = "https://checkip.amazonaws.com"
}

locals {
  my_ip_cidr = "${chomp(data.http.myip.response_body)}/32"
}

resource "aws_security_group" "web" {
  name        = "${var.name_prefix}-web-sg"
  description = "SSH from my IP, HTTP from anywhere"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "SSH from my IP"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [local.my_ip_cidr]
  }

  ingress {
    description = "HTTP from anywhere"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    description = "All outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.name_prefix}-web-sg" }
}
