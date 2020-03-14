terraform {
  backend "remote" {
    hostname     = "app.terraform.io"
    organization = "Quames"

    workspaces {
      name = "manatee-twitter"
    }
  }
}

provider "aws" {
  region = "us-east-1"
}

data "aws_s3_bucket_object" "zip_hash" {
  bucket = "james-lambda-builds"
  key    = "manatee-twitter/manatee-twitter.zip.base64sha256"
}

resource "aws_lambda_function" "manatee_twitter" {
  s3_bucket     = "james-lambda-builds"
  s3_key        = "manatee-twitter/manatee-twitter.zip"
  function_name = "manatee-twitter"
  role          = aws_iam_role.manatee_twitter.arn
  handler       = "manatee-twitter"

  runtime = "go1.x"

  source_code_hash = data.aws_s3_bucket_object.zip_hash.body

  lifecycle {
    ignore_changes = [
      environment,
    ]
  }
}

resource "aws_cloudwatch_event_rule" "manatee_twitter" {
  name                = "manatee-twitter-invocation"
  description         = "Runs the manatee-twitter bot daily"
  schedule_expression = "cron(0 13 * * ? *)"
}

resource "aws_cloudwatch_event_target" "manatee_twitter" {
  rule      = aws_cloudwatch_event_rule.manatee_twitter.name
  target_id = "manatee_twitter"
  arn       = aws_lambda_function.manatee_twitter.arn
}

resource "aws_lambda_permission" "manatee_twitter" {
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.manatee_twitter.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.manatee_twitter.arn
}

data "aws_iam_policy_document" "manatee_twitter_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }

}

data "aws_iam_policy_document" "manatee_twitter_permissions" {
  statement {
    actions = [
      "s3:*"
    ]
    resources = [
      "arn:aws:s3:::quaki-manatee-pics",
      "arn:aws:s3:::quaki-manatee-pics/*"
    ]
  }
}

resource "aws_iam_role" "manatee_twitter" {
  name               = "manatee-twitter"
  assume_role_policy = data.aws_iam_policy_document.manatee_twitter_assume_role.json
}

resource "aws_iam_policy" "manatee_twitter_policy" {
  policy = data.aws_iam_policy_document.manatee_twitter_permissions.json
}


resource "aws_iam_role_policy_attachment" "manatee_twitter_policy" {
  role       = aws_iam_role.manatee_twitter.name
  policy_arn = aws_iam_policy.manatee_twitter_policy.arn
}

resource "aws_iam_policy" "manatee_twitter_lambda_logging" {
  name        = "manatee_twitter_lambda_logging"
  path        = "/"
  description = "IAM policy for logging from a lambda"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents"
      ],
      "Resource": "arn:aws:logs:*:*:*",
      "Effect": "Allow"
    }
  ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "manatee_twitter_lambda_logs" {
  role       = aws_iam_role.manatee_twitter.name
  policy_arn = aws_iam_policy.manatee_twitter_lambda_logging.arn
}
