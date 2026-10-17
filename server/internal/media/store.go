package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Store interface {
	PresignPut(ctx context.Context, key, mime string, size int64, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Get(ctx context.Context, key string, max int64) ([]byte, error)
	Put(ctx context.Context, key, mime string, b []byte) error
	Delete(ctx context.Context, key string) error
}

type R2 struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func NewR2(accountID, accessKey, secret, bucket string) *R2 {
	c := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)),
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secret, ""),
		UsePathStyle: true,
	})
	return &R2{client: c, presign: s3.NewPresignClient(c), bucket: bucket}
}

// The signature covers content type and length, so the client can't upload anything else.
func (r *R2) PresignPut(ctx context.Context, key, mime string, size int64, ttl time.Duration) (string, error) {
	req, err := r.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: &r.bucket, Key: &key, ContentType: &mime, ContentLength: &size,
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (r *R2) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := r.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &r.bucket, Key: &key}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (r *R2) Get(ctx context.Context, key string, max int64) ([]byte, error) {
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &r.bucket, Key: &key})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	b, err := io.ReadAll(io.LimitReader(out.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, ErrTooLarge
	}
	return b, nil
}

func (r *R2) Put(ctx context.Context, key, mime string, b []byte) error {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &r.bucket, Key: &key, ContentType: &mime, Body: bytes.NewReader(b)})
	return err
}

func (r *R2) Delete(ctx context.Context, key string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &r.bucket, Key: &key})
	return err
}

// Memory is the store for local development and tests.
type Memory struct {
	mu      sync.Mutex
	Objects map[string][]byte
	Base    string
}

func NewMemory(base string) *Memory { return &Memory{Objects: map[string][]byte{}, Base: base} }

func (m *Memory) PresignPut(_ context.Context, key, _ string, _ int64, _ time.Duration) (string, error) {
	return m.Base + "/dev-upload/" + key, nil
}

func (m *Memory) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return m.Base + "/dev-media/" + key, nil
}

func (m *Memory) Get(_ context.Context, key string, max int64) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.Objects[key]
	if !ok {
		return nil, ErrNotFound
	}
	if int64(len(b)) > max {
		return nil, ErrTooLarge
	}
	return b, nil
}

func (m *Memory) Put(_ context.Context, key, _ string, b []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Objects[key] = b
	return nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Objects, key)
	return nil
}
