package blob

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"regexp"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	azureblob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
)

type azureBackend struct {
	client *azblob.Client
	bucket string
}

var accountPattern = regexp.MustCompile(`^[a-z0-9]{3,24}$`)

func newAzure(c Config) (backend, error) {
	if c.Bucket == "" {
		return nil, errors.New("Azure container is required")
	}
	if c.Endpoint != "" {
		if _, err := endpoint(c.Endpoint); err != nil {
			return nil, err
		}
	}
	opts := &azblob.ClientOptions{}
	opts.Retry.MaxRetries = 1
	var client *azblob.Client
	var err error
	if c.ConnectionString != "" {
		if c.Endpoint != "" || c.AccountKey != "" {
			return nil, errors.New("choose Azure connection string or account credentials")
		}
		client, err = azblob.NewClientFromConnectionString(c.ConnectionString, opts)
	} else {
		if !accountPattern.MatchString(c.AccountName) || c.AccountKey == "" {
			return nil, errors.New("Azure account name and key are required")
		}
		credential, credentialErr := azblob.NewSharedKeyCredential(c.AccountName, c.AccountKey)
		if credentialErr != nil {
			return nil, errors.New("invalid Azure account credentials")
		}
		target := c.Endpoint
		if target == "" {
			target = "https://" + c.AccountName + ".blob.core.windows.net"
		}
		client, err = azblob.NewClientWithSharedKeyCredential(target, credential, opts)
	}
	if err != nil {
		return nil, errors.New("cannot initialize Azure credentials")
	}
	return &azureBackend{client: client, bucket: c.Bucket}, nil
}
func azureError(err error) error {
	var response *azcore.ResponseError
	if errors.As(err, &response) && response.ErrorCode == "BlobNotFound" {
		return fs.ErrNotExist
	}
	return err
}
func (b *azureBackend) put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := b.client.UploadStream(ctx, b.bucket, key, &exactReader{r: r, remaining: size}, &azblob.UploadStreamOptions{BlockSize: 4 * 1024 * 1024, Concurrency: 2, HTTPHeaders: &azureblob.HTTPHeaders{BlobContentType: &contentType}})
	return azureError(err)
}
func (b *azureBackend) open(ctx context.Context, key string) (io.ReadCloser, error) {
	response, err := b.client.DownloadStream(ctx, b.bucket, key, nil)
	if err != nil {
		return nil, azureError(err)
	}
	return response.Body, nil
}
func (b *azureBackend) delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteBlob(ctx, b.bucket, key, nil)
	return azureError(err)
}
