package blob

import (
	"context"
	"errors"
	"io"
	"io/fs"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type s3Backend struct {
	client *s3.Client
	bucket string
}

func newS3(c Config) (backend, error) {
	if c.Bucket == "" {
		return nil, errors.New("S3 bucket is required")
	}
	if (c.AccessKeyID == "") != (c.SecretAccessKey == "") || (c.SessionToken != "" && c.AccessKeyID == "") {
		return nil, errors.New("S3 credentials are incomplete")
	}
	if c.Endpoint != "" {
		if _, err := endpoint(c.Endpoint); err != nil {
			return nil, err
		}
	}
	region := c.Region
	if region == "" {
		region = "us-east-1"
	}
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if c.AccessKeyID != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKeyID, c.SecretAccessKey, c.SessionToken)))
	}
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, errors.New("cannot initialize S3 credentials")
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if c.Endpoint != "" {
			o.BaseEndpoint = aws.String(c.Endpoint)
			o.UsePathStyle = true
		}
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		o.RetryMaxAttempts = 2
	})
	return &s3Backend{client: client, bucket: c.Bucket}, nil
}
func s3Error(err error) error {
	var api smithy.APIError
	if errors.As(err, &api) && (api.ErrorCode() == "NoSuchKey" || api.ErrorCode() == "NotFound") {
		return fs.ErrNotExist
	}
	return err
}
func (b *s3Backend) put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(b.bucket), Key: aws.String(key), Body: &exactReader{r: r, remaining: size}, ContentLength: aws.Int64(size), ContentType: aws.String(contentType)}, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
	})
	return s3Error(err)
}
func (b *s3Backend) open(ctx context.Context, key string) (io.ReadCloser, error) {
	response, err := b.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(b.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, s3Error(err)
	}
	return response.Body, nil
}
func (b *s3Backend) delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(b.bucket), Key: aws.String(key)})
	return s3Error(err)
}
