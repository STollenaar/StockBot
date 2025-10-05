resource "aws_iam_role" "stockbot_role" {
  name               = "stockbotRole"
  description        = "Role for the stockbot"
  assume_role_policy = data.aws_iam_policy_document.assume_policy_document.json
}

resource "aws_iam_role_policy" "stockbot_role_policy" {
  role   = aws_iam_role.stockbot_role.id
  name   = "inline-role"
  policy = data.aws_iam_policy_document.ssm_access_role_policy_document.json
}
