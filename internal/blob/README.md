# Private blob adapters

`New(Config)` implements the storage interface in `docs/API.md`. Selecting `local`
initializes only filesystem storage; cloud SDK credential discovery and requests
are confined to the selected cloud provider. The caller supplies the local root.
The server retains MIME metadata for local files; cloud writes pass ContentType
to the provider SDK. No adapter creates a public bucket, changes ACLs, or emits
credentials through error strings.

Configuration:

- `local`: `Root` is required. Files use `os.Root` confinement, private modes,
  temporary files, size validation, file sync and atomic rename.
- `s3`: `Bucket`; optional `Region` (defaults to `us-east-1`) and service-base
  `Endpoint`. A custom endpoint enables path-style addressing for compatible
  servers. Complete `AccessKeyID` / `SecretAccessKey` credentials override the
  AWS default chain; `SessionToken` is supported. PutObject accepts a streaming,
  non-seekable reader using the official unsigned-payload signing middleware.
- `gcs`: `Bucket`; optional `ServiceAccountJSON` containing the credential JSON,
  otherwise Application Default Credentials. An explicit `Endpoint` is a base
  service URL, without `/storage/v1`. Only an explicit loopback endpoint without
  credentials selects unauthenticated transport. The official SDK additionally
  honors its `STORAGE_EMULATOR_HOST` convention. Client metrics are disabled.
- `azure`: `Bucket` means container. Supply either `ConnectionString`, or
  `AccountName` / base64 `AccountKey` with optional service-base `Endpoint`.
  Connection-string credentials cannot be combined with a separate endpoint or
  account key. Custom Azure endpoints may contain the emulator account path.

A prefix's outer slashes are removed consistently across all adapters. Object
keys and prefixes reject traversal, empty path segments, control characters,
backslashes and URL syntax. Buckets/containers reject path and URL injection;
the provider still enforces its complete naming rules. Combined keys are capped
at 1,024 bytes. Put requires an exact nonnegative byte count. Failed local writes
preserve an existing object. A failed cloud operation can have provider-side
partial state, so the server must roll back uncommitted artifact keys.

Operations have a five-minute context deadline and preserve an earlier caller
deadline. Open retains its context through streaming and cancels on EOF/Close.
Callers must close returned readers. Cancellation can interrupt provider I/O;
an arbitrary caller-owned Reader must itself cooperate if its Read blocks.
`Test` writes random bytes under a random private probe key, reads and compares
bytes, then deletes the key. Its main deadline is 30 seconds, with independent
five-second cleanup after success, failure, or cancellation. A cleanup failure
fails the test. An already canceled probe makes no provider requests.

## Validation

`go test -race ./internal/blob` runs real filesystem tests and official SDKs
against strict HTTP contract emulators for S3, GCS JSON/resumable upload and
Azure direct/block upload. It checks credentials, content types, streaming,
malicious keys, cancellation, missing objects, redaction, corruption and probe
cleanup. Independent local emulators can be selected explicitly:

```sh
FOLIO_TEST_AZURITE=http://127.0.0.1:38471/folioaccount \
FOLIO_TEST_GCS=http://127.0.0.1:38472 \
go test -race ./internal/blob -run TestStandaloneEmulators -v
```

Azurite must be configured with the public test-only account
`AZURITE_ACCOUNTS=folioaccount:dGVzdC1vbmx5LWtleQ==`. Start fake-gcs-server with
HTTP and the matching `-external-url`. Optional `FOLIO_TEST_S3` selects a loopback
MinIO using its public test credentials `minioadmin/minioadmin`. Standalone tests
create and remove their own private bucket/container, stream approximately
9.45 MB, and refuse non-loopback endpoints. Unselected standalone tests skip.
These emulators do not establish live AWS, R2, GCP or Azure account behavior.

## References checked

- [AWS official S3 Go examples](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/go_s3_code_examples.html)
- [GCS official Go upload example](https://cloud.google.com/storage/docs/uploading-objects-from-memory)
- [Azure official Go upload guide](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blob-upload-go)
- [Go os.Root reference](https://pkg.go.dev/os#Root)
- [Azurite reference](https://github.com/Azure/Azurite)
- [fake-gcs-server reference](https://github.com/fsouza/fake-gcs-server)

Actual pinned source/type definitions inspected: AWS S3 v1.114.10 `options.go`
(`BaseEndpoint`, path style, checksum/retry options); AWS core v1.48.0
`aws/signer/v4/middleware.go:197` (unsigned payload middleware); GCS v1.70.0
`storage.go:152` (HTTP client/auth/emulator setup), `option.go:135` (JSON reads),
`option.go:175` (disabled metrics), `writer.go:552` (abort lifecycle); Azure azblob
v1.8.3 `client.go:60` / `:74` / `:141` / `:158` (constructors and stream methods),
`blockblob/models.go:490` (bounded block buffers and content headers).
