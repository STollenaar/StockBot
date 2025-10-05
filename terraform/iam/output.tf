output "iam" {
  value = {
    stockbot_role = aws_iam_role.stockbot_role
  }
}
