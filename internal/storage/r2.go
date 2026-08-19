package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var ErrNotConfigured = errors.New("storage not configured")

// R2 Cloudflare R2（S3 兼容）实现
type R2 struct {
	client *s3.Client
	bucket string
}

func NewR2(endpoint, accessKey, secretKey, bucket string) (*R2, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: endpoint, HostnameImmutable: true}, nil
		})),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	return &R2{client: client, bucket: bucket}, nil
}

func (r *R2) Put(key string, data []byte, contentType string, cacheMaxAge int) error {
	_, err := r.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:       aws.String(r.bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(data),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String(fmt.Sprintf("public, max-age=%d, immutable", cacheMaxAge)),
	})
	return err
}

func (r *R2) Delete(keys []string) error {
	for len(keys) > 0 {
		n := len(keys)
		if n > 1000 {
			n = 1000
		}
		objs := make([]types.ObjectIdentifier, 0, n)
		for _, k := range keys[:n] {
			objs = append(objs, types.ObjectIdentifier{Key: aws.String(k)})
		}
		keys = keys[n:]
		out, err := r.client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{
			Bucket: aws.String(r.bucket),
			Delete: &types.Delete{Objects: objs},
		})
		if err != nil {
			return err
		}
		for _, e := range out.Errors {
			if e.Key != nil {
				return fmt.Errorf("delete failed for %s: %s", *e.Key, aws.ToString(e.Message))
			}
		}
	}
	return nil
}

func (r *R2) list(prefix string, fn func(ObjectInfo) error) error {
	var token *string
	for {
		in := &s3.ListObjectsV2Input{
			Bucket:            aws.String(r.bucket),
			ContinuationToken: token,
		}
		if prefix != "" {
			in.Prefix = aws.String(prefix)
		}
		out, err := r.client.ListObjectsV2(context.Background(), in)
		if err != nil {
			return err
		}
		for _, o := range out.Contents {
			if o.Key == nil {
				continue
			}
			info := ObjectInfo{Key: *o.Key, Size: aws.ToInt64(o.Size)}
			if o.LastModified != nil {
				info.LastModified = *o.LastModified
			}
			if err := fn(info); err != nil {
				return err
			}
		}
		if !aws.ToBool(out.IsTruncated) || out.NextContinuationToken == nil {
			break
		}
		token = out.NextContinuationToken
	}
	return nil
}

func (r *R2) ListAll(fn func(ObjectInfo) error) error {
	return r.list("", fn)
}

func (r *R2) ListPrefix(prefix string, fn func(ObjectInfo) error) error {
	return r.list(prefix, fn)
}

// Ping 校验连通性（List 一页）
func (r *R2) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := r.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(r.bucket),
		MaxKeys: aws.Int32(1),
	})
	return err
}
