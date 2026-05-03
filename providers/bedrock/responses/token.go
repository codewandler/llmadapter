package responses

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/codewandler/llmadapter/providers/bedrock/internal/mantleauth"
)

func NewAWSTokenProvider(region string) *AWSTokenProvider {
	return mantleauth.NewAWSTokenProvider(region)
}

func GenerateToken(ctx context.Context, creds aws.Credentials, region string, expiresInSeconds int, signingTime time.Time) (string, error) {
	return mantleauth.GenerateToken(ctx, creds, region, expiresInSeconds, signingTime)
}

func explicitTokenFromEnv() string {
	return mantleauth.ExplicitTokenFromEnv()
}
