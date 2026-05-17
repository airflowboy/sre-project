# Ch 10 Phase D — AWS Secrets Manager (ADR-014).
#
# Two secrets so far:
#   - db_password   plain string (the password)
#   - db_url        full DSN string (host/port/db baked in) for convenience
#
# Pods fetch these via IRSA (see iam_app.tf) — no static AWS keys in the cluster.
#
# recovery_window_in_days = 0 → immediate delete on destroy. AWS default is 30,
# which would leave residue and block re-creating with the same name within
# that window — annoying for daily learning destroy/apply cycles.

resource "aws_secretsmanager_secret" "db_password" {
  name                    = "${var.cluster_name}/db/password"
  description             = "PostgreSQL password for ${var.db_username}"
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "db_password" {
  secret_id     = aws_secretsmanager_secret.db_password.id
  secret_string = random_password.db.result
}

# Convenience: full DSN string so the app does not need to assemble one
# from individual fields. Format that lib/pq and pgx both accept.
resource "aws_secretsmanager_secret" "db_url" {
  name                    = "${var.cluster_name}/db/url"
  description             = "PostgreSQL DSN for issue-api"
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "db_url" {
  secret_id = aws_secretsmanager_secret.db_url.id
  secret_string = format(
    "postgres://%s:%s@%s:%d/%s?sslmode=require",
    var.db_username,
    random_password.db.result,
    aws_db_instance.postgres.address,
    aws_db_instance.postgres.port,
    var.db_name,
  )
}
