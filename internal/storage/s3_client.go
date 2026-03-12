package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type S3Config struct {
	Region     string
	Bucket     string
	AccessKey  string
	SecretKey  string
	Endpoint   string
	PublicBase string
}

type Client struct {
	cfg S3Config
	s3  *s3.Client
}

func NewClient(ctx context.Context, cfg S3Config) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	hasAccessKey := strings.TrimSpace(cfg.AccessKey) != ""
	hasSecretKey := strings.TrimSpace(cfg.SecretKey) != ""
	if hasAccessKey != hasSecretKey {
		return nil, errors.New("s3 access key and secret key must be provided together")
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint != "" {
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid s3 endpoint: %w", err)
		}
		endpoint = u.String()
	}

	var awsCfg aws.Config
	if hasAccessKey {
		awsCfg = aws.Config{
			Region:      cfg.Region,
			Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		}
	} else {
		var err error
		awsCfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
		if err != nil {
			return nil, err
		}
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint == "" {
			return
		}
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &Client{
		cfg: cfg,
		s3:  s3Client,
	}, nil
}

func (c *Client) PutObject(ctx context.Context, key, contentType string, body io.Reader, sizeBytes int64) error {
	if c == nil {
		return errors.New("s3 client not initialized")
	}
	key = strings.TrimSpace(strings.TrimPrefix(key, "/"))
	if key == "" {
		return errors.New("object key is required")
	}
	contentType = strings.TrimSpace(contentType)
	if err := c.ValidateContentType(contentType); err != nil {
		return err
	}
	if body == nil {
		return errors.New("object body is required")
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.cfg.Bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}
	if sizeBytes > 0 {
		input.ContentLength = aws.Int64(sizeBytes)
	}

	_, err := c.s3.PutObject(ctx, input)
	return err
}

func (c *Client) FileURL(key string) string {
	if c == nil {
		return ""
	}
	if strings.TrimSpace(c.cfg.PublicBase) == "" {
		return ""
	}
	key = strings.TrimSpace(strings.TrimPrefix(key, "/"))
	if key == "" {
		return ""
	}
	joined, err := url.JoinPath(c.cfg.PublicBase, key)
	if err != nil {
		return ""
	}
	return joined
}

func (c *Client) ObjectExists(ctx context.Context, key string) (bool, error) {
	if c == nil {
		return false, errors.New("s3 client not initialized")
	}

	key = strings.TrimSpace(strings.TrimPrefix(key, "/"))
	if key == "" {
		return false, errors.New("object key is required")
	}

	_, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return false, nil
		}
	}

	return false, err
}

func (c *Client) ValidateContentType(contentType string) error {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return errors.New("content type is required")
	}
	if _, _, err := mime.ParseMediaType(contentType); err != nil {
		return fmt.Errorf("invalid content type: %w", err)
	}
	return nil
}

func (cfg S3Config) validate() error {
	if strings.TrimSpace(cfg.Region) == "" {
		return errors.New("s3 region is required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return errors.New("s3 bucket is required")
	}
	return nil
}
