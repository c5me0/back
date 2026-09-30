package config

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	Endpoint string `json:"endpoint"`
	// PublicEndpoint is the host clients reach for presigned URLs. Defaults to Endpoint.
	PublicEndpoint string `json:"public_endpoint"`
	Bucket         string `json:"bucket"`
	AccessKey      string `json:"access_key"`
	SecretKey      string `json:"secret_key"`
	Region         string `json:"region"`
	Insecure       bool   `json:"insecure"`
}

func (s *Storage) Validate() error {
	if s.Region == "" {
		s.Region = "us-east-1"
	}

	return validation.ValidateStruct(s,
		validation.Field(&s.Endpoint, validation.Required),
		validation.Field(&s.Bucket, validation.Required),
		validation.Field(&s.AccessKey, validation.Required),
		validation.Field(&s.SecretKey, validation.Required),
	)
}

// Client builds the client used for server-side object operations.
func (s *Storage) Client() (*minio.Client, error) {
	return s.client(s.Endpoint)
}

// PublicClient builds the client used to presign URLs handed to clients.
// The region is fixed so presigning never probes the endpoint, which may be unreachable from the server.
func (s *Storage) PublicClient() (*minio.Client, error) {
	if s.PublicEndpoint == "" {
		return s.client(s.Endpoint)
	}
	return s.client(s.PublicEndpoint)
}

func (s *Storage) client(endpoint string) (*minio.Client, error) {
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s.AccessKey, s.SecretKey, ""),
		Secure: !s.Insecure,
		Region: s.Region,
	})
}
