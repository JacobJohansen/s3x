package s3x

import (
	"errors"

	minio "github.com/minio/minio/cmd"
)

var (
	// ErrLedgerBucketExists is an error message returned from the internal
	// ledgerStore indicating that a bucket already exists
	ErrLedgerBucketExists = errors.New("bucket exists")
	// ErrLedgerBucketDoesNotExist is an error message returned from the internal
	// ledgerStore indicating that a bucket does not exist
	ErrLedgerBucketDoesNotExist = errors.New("bucket does not exist")
	// ErrLedgerObjectDoesNotExist is an error message returned from the internal
	// ledgerStore indicating that a object does not exist
	ErrLedgerObjectDoesNotExist = errors.New("object does not exist")
	// ErrLedgerNonEmptyBucket is an error message returned from the internal
	// ledgerStore indicating that a bucket is not empty
	ErrLedgerNonEmptyBucket = errors.New("bucket is not empty")
	// ErrInvalidUploadID is an error message returned when the multipart upload id
	// does not exist
	ErrInvalidUploadID = errors.New("invalid multipart upload id")
	// ErrInvalidPartNumber is an error message returned when the multipart part
	// number is out of range (not mappable to a minio error type)
	ErrInvalidPartNumber = errors.New("invalid multipart part number")
)

// toMinioErr converts gRPC or ledger errors into compatible minio errors
// or if no error is present return nil
func (x *xObjects) toMinioErr(err error, bucket, object, id string) error {
	switch err {
	case ErrLedgerBucketDoesNotExist:
		err = minio.BucketNotFound{Bucket: bucket}
	case ErrLedgerObjectDoesNotExist:
		err = minio.ObjectNotFound{Bucket: bucket, Object: object}
	case ErrLedgerBucketExists:
		err = minio.BucketAlreadyExists{Bucket: bucket}
	case ErrInvalidUploadID:
		err = minio.InvalidUploadID{Bucket: bucket, Object: object, UploadID: id}
	case ErrLedgerNonEmptyBucket:
		err = minio.BucketNotEmpty{Bucket: bucket}
	case nil:
		return nil
	}
	return err
}
