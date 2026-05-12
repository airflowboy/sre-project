# Phase C — an EC2 instance in the public subnet from network.tf.
#
# This is the integration test for "the 4 things that make a public subnet":
# if SSH and HTTP both work, then subnet + IGW + route + public-IP are all wired.
#
# user_data (cloud-init) installs nginx so we have something to curl.
# data "aws_ami" looks up the latest Amazon Linux 2023 image — never hardcode AMI IDs.

# Register our local SSH public key as an EC2 key pair.
# file() does NOT expand "~", so wrap the path in pathexpand().
resource "aws_key_pair" "main" {
  key_name   = "${var.name_prefix}-key"
  public_key = file(pathexpand(var.public_key_path))
}

data "aws_ami" "al2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-2023.*-x86_64"]
  }
  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_instance" "web" {
  ami                    = data.aws_ami.al2023.id
  instance_type          = var.instance_type
  subnet_id              = aws_subnet.public.id
  vpc_security_group_ids = [aws_security_group.web.id]
  key_name               = aws_key_pair.main.key_name

  user_data = <<-EOT
    #!/bin/bash
    dnf install -y nginx
    echo "<h1>hello from $(hostname) — Terraform Ch08</h1>" > /usr/share/nginx/html/index.html
    systemctl enable --now nginx
  EOT

  tags = { Name = "${var.name_prefix}-web" }
}
