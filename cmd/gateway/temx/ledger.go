package temx

import (
	"errors"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
)

var (
	ledgerKey    = datastore.NewKey("ledgerstatekey")
	ledgerPrefix = datastore.NewKey("ledgerRoot")
	// errors

	// ErrLedgerBucketExists is an error message returned from the internal
	// ledgerStore indicating that a bucket already exists
	ErrLedgerBucketExists = errors.New("bucket exists")
	// ErrLedgerBucketDoesNotExist is an error message returned from the internal
	// ledgerStore indicating that a bucket does not exist
	ErrLedgerBucketDoesNotExist = errors.New("bucket does not exist")
	// ErrLedgerObjectDoesNotExist is an error message returned from the internal
	// ledgerStore indicating that a object does not exist
	ErrLedgerObjectDoesNotExist = errors.New("object does not exist")
)

type ledgerStore struct {
	locker *sync.RWMutex
	ds     datastore.Batching
}

func newLedgerStore(ds datastore.Batching) *ledgerStore {
	ledger := &ledgerStore{
		locker: &sync.RWMutex{},
		ds:     namespace.Wrap(ds, ledgerPrefix),
	}
	ledger.createIfNotExist()
	return ledger
}

func (le *ledgerStore) NewBucket(name, hash string) error {
	le.locker.Lock()
	defer le.locker.Unlock()
	ledger, err := le.getLedger()
	if err != nil {
		return err
	}
	if le.bucketExists(ledger, name) {
		return ErrLedgerBucketExists
	}
	if ledger.GetBuckets() == nil {
		ledger.Buckets = make(map[string]*LedgerBucketEntry)
	}
	ledger.Buckets[name] = &LedgerBucketEntry{
		Objects:  make(map[string]*LedgerObjectEntry),
		Name:     name,
		IpfsHash: hash,
	}
	return le.putLedger(ledger)
}

func (le *ledgerStore) UpdateBucketHash(name, hash string) error {
	le.locker.Lock()
	defer le.locker.Unlock()
	ledger, err := le.getLedger()
	if err != nil {
		return err
	}
	if !le.bucketExists(ledger, name) {
		return ErrLedgerBucketDoesNotExist
	}
	ledger.Buckets[name].IpfsHash = hash
	return le.putLedger(ledger)
}

func (le *ledgerStore) RemoveObject(bucketName, objectName string) error {
	le.locker.Lock()
	defer le.locker.Unlock()
	ledger, err := le.getLedger()
	if err != nil {
		return err
	}
	if err := le.objectExists(ledger, bucketName, objectName); err != nil {
		return err
	}
	delete(ledger.Buckets[bucketName].Objects, objectName)
	return nil
}

func (le *ledgerStore) AddObjectToBucket(bucketName, objectName, objectHash string) error {
	le.locker.Lock()
	defer le.locker.Unlock()
	ledger, err := le.getLedger()
	if err != nil {
		return err
	}
	if !le.bucketExists(ledger, bucketName) {
		return ErrLedgerBucketDoesNotExist
	}
	// prevent nil map panic
	if ledger.GetBuckets()[bucketName].GetObjects() == nil {
		ledger.Buckets[bucketName].Objects = make(map[string]*LedgerObjectEntry)
	}
	ledger.Buckets[bucketName].Objects[objectName] = &LedgerObjectEntry{
		Name:     objectName,
		IpfsHash: objectHash,
	}
	return le.putLedger(ledger)
}

func (le *ledgerStore) BucketExists(name string) bool {
	le.locker.RLock()
	defer le.locker.RUnlock()
	ledger, err := le.getLedger()
	if err != nil {
		return false
	}
	return le.bucketExists(ledger, name)
}
func (le *ledgerStore) GetBucketHash(name string) (string, error) {
	le.locker.RLock()
	defer le.locker.RUnlock()
	ledger, err := le.getLedger()
	if err != nil {
		return "", err
	}
	if ledger.GetBuckets()[name] == nil {
		return "", ErrLedgerBucketDoesNotExist
	}
	return ledger.Buckets[name].GetIpfsHash(), nil
}

func (le *ledgerStore) GetObjectHashFromBucket(bucketName, objectName string) (string, error) {
	le.locker.RLock()
	defer le.locker.RUnlock()
	ledger, err := le.getLedger()
	if err != nil {
		return "", err
	}
	if ledger.GetBuckets()[bucketName] == nil {
		return "", ErrLedgerBucketDoesNotExist
	}
	bucket := ledger.GetBuckets()[bucketName]
	if bucket.GetObjects()[objectName] == nil {
		return "", ErrLedgerObjectDoesNotExist
	}
	return bucket.GetObjects()[objectName].GetIpfsHash(), nil
}

func (le *ledgerStore) DeleteBucket(name string) error {
	le.locker.Lock()
	defer le.locker.Unlock()
	ledger, err := le.getLedger()
	if err != nil {
		return err
	}
	if ledger.GetBuckets()[name] == nil {
		return ErrLedgerBucketDoesNotExist
	}
	delete(ledger.Buckets, name)
	return le.putLedger(ledger)
}

func (le *ledgerStore) GetObjectHashes(bucket string) (map[string]string, error) {
	le.locker.RLock()
	defer le.locker.RUnlock()
	ledger, err := le.getLedger()
	if err != nil {
		return nil, err
	}
	if !le.bucketExists(ledger, bucket) {
		return nil, ErrLedgerBucketDoesNotExist
	}
	// maps object names to hashes
	var hashes = make(map[string]string, len(ledger.Buckets[bucket].Objects))
	for _, obj := range ledger.GetBuckets()[bucket].GetObjects() {
		hashes[obj.GetName()] = obj.GetIpfsHash()
	}
	return hashes, err
}

func (le *ledgerStore) GetBucketNames() ([]string, error) {
	le.locker.RLock()
	defer le.locker.RUnlock()
	ledger, err := le.getLedger()
	if err != nil {
		return nil, err
	}
	var (
		// maps bucket names to hashes
		names = make([]string, len(ledger.Buckets))
		count int
	)
	for _, b := range ledger.Buckets {
		names[count] = b.GetName()
		count++
	}
	return names, nil
}

// these functions are never to be accessed directly and instead through the public functions

func (le *ledgerStore) getLedger() (*Ledger, error) {
	ledgerBytes, err := le.ds.Get(ledgerKey)
	if err != nil {
		return nil, err
	}
	ledger := &Ledger{}
	if err := ledger.Unmarshal(ledgerBytes); err != nil {
		return nil, err
	}
	return ledger, nil
}

func (le *ledgerStore) createIfNotExist() {
	if _, err := le.getLedger(); err == nil {
		return
	}
	ledger := new(Ledger)
	ledgerBytes, err := ledger.Marshal()
	if err != nil {
		panic(err)
	}
	if err := le.ds.Put(ledgerKey, ledgerBytes); err != nil {
		panic(err)
	}
}

func (le *ledgerStore) objectExists(ledger *Ledger, bucket, object string) error {
	if ledger.GetBuckets()[bucket] == nil {
		return ErrLedgerBucketDoesNotExist
	}
	if ledger.GetBuckets()[bucket].GetObjects()[object] == nil {
		return ErrLedgerObjectDoesNotExist
	}
	return nil
}

func (le *ledgerStore) bucketExists(ledger *Ledger, name string) bool {
	return ledger.GetBuckets()[name] != nil
}

func (le *ledgerStore) putLedger(ledger *Ledger) error {
	ledgerBytes, err := ledger.Marshal()
	if err != nil {
		return err
	}
	return le.ds.Put(ledgerKey, ledgerBytes)
}