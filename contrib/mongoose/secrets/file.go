package secrets

import (
	"context"
	"os"

	"github.com/joho/godotenv"
)

type FileStorage struct {
	secrets map[string]string
}

func NewFile(path string) (Storage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	envs, err := godotenv.Parse(file)
	if err != nil {
		return nil, err
	}

	return &FileStorage{
		secrets: envs,
	}, nil
}

func (c *FileStorage) GetSecret(_ context.Context, key string) (Secret, error) {
	secret, ok := c.secrets[key]
	if !ok {
		return nil, ErrSecretNotFound
	}

	return &plainSecret{data: []byte(secret)}, nil
}

func (c *FileStorage) Close() error { return nil }
