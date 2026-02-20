package storage

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Config struct {
	Region     string
	Bucket     string
	AccessKey  string
	SecretKey  string
	Endpoint   string
	PublicBase string
	PresignTTL time.Duration
}

type Client struct {
	cfg     S3Config
	s3      *s3.Client
	presign *s3.PresignClient
}

func NewClient(ctx context.Context, cfg S3Config) (*Client, error) {
	if cfg.Region == "" || cfg.Bucket == "" {
		return nil, errors.New("s3 region and bucket are required")
	}

	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(cfg.Region))

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")))
	}

	if cfg.Endpoint != "" {
		endpoint := cfg.Endpoint
		if parsed, err := url.Parse(endpoint); err == nil {
			endpoint = parsed.String()
		}
		opts = append(opts, config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				if service == s3.ServiceID {
					return aws.Endpoint{URL: endpoint, SigningRegion: cfg.Region}, nil
				}
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			}),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.UsePathStyle = true
		}
	})
	presignClient := s3.NewPresignClient(s3Client)

	return &Client{
		cfg:     cfg,
		s3:      s3Client,
		presign: presignClient,
	}, nil
}

func (c *Client) PresignPut(ctx context.Context, key, contentType string, sizeBytes int64) (string, map[string]string, error) {
	if c == nil {
		return "", nil, errors.New("s3 client not initialized")
	}
	if key == "" {
		return "", nil, errors.New("object key is required")
	}
	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.cfg.Bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}
	if sizeBytes > 0 {
		input.ContentLength = aws.Int64(sizeBytes)
	}

	presigned, err := c.presign.PresignPutObject(ctx, input, func(po *s3.PresignOptions) {
		if c.cfg.PresignTTL > 0 {
			po.Expires = c.cfg.PresignTTL
		}
	})
	if err != nil {
		return "", nil, err
	}

	headers := map[string]string{}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	if sizeBytes > 0 {
		headers["Content-Length"] = strconv.FormatInt(sizeBytes, 10)
	}

	return presigned.URL, headers, nil
}

func (c *Client) FileURL(key string) string {
	if c == nil || key == "" {
		return ""
	}
	if c.cfg.PublicBase != "" {
		return c.cfg.PublicBase + "/" + key
	}
	return ""
}

func (c *Client) ValidateContentType(contentType string) error {
	if contentType == "" {
		return errors.New("content type is required")
	}
	return nil
}

func (c *Client) ValidateACL(acl string) (types.ObjectCannedACL, error) {
	if acl == "" {
		return types.ObjectCannedACLPrivate, nil
	}
	switch acl {
	case "private":
		return types.ObjectCannedACLPrivate, nil
	case "public-read":
		return types.ObjectCannedACLPublicRead, nil
	default:
		return "", errors.New("invalid acl")
	}
}
