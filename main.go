package main

import (
	"github.com/James-Quigley/manatee-twitter/internal"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(internal.Handle)
}
