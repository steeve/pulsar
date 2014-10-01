package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"time"
)

type FileStore struct {
	path string
}

type fileStoreItem struct {
	Key     string      `json:"key"`
	Value   interface{} `json:"value"`
	Expires time.Time   `json:"expires"`
}

func NewFileStore(path string) *FileStore {
	os.MkdirAll(path, 0766)
	return &FileStore{path}
}

func (c *FileStore) Set(key string, value interface{}, expires time.Duration) error {
	filename := path.Join(c.path, key)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	item := fileStoreItem{
		Key:     key,
		Value:   value,
		Expires: time.Now().Local().Add(expires),
	}

	return json.NewEncoder(file).Encode(item)
}

func (c *FileStore) Add(key string, value interface{}, expires time.Duration) error {
	if _, err := os.Stat(path.Join(c.path, key)); os.IsExist(err) {
		return os.ErrExist
	}
	return c.Set(key, value, expires)
}

func (c *FileStore) Replace(key string, value interface{}, expires time.Duration) error {
	if _, err := os.Stat(path.Join(c.path, key)); os.IsNotExist(err) {
		return os.ErrNotExist
	}
	return c.Set(key, value, expires)
}

func (c *FileStore) Get(key string, value interface{}) error {
	file, err := os.Open(path.Join(c.path, key))
	if err != nil {
		return err
	}
	defer file.Close()

	item := fileStoreItem{
		Value: value,
	}
	if err = json.NewDecoder(file).Decode(&item); err != nil {
		return err
	}
	if item.Expires.Before(time.Now()) {
		return errors.New("key is expired")
	}
	return nil
}

func (c *FileStore) Delete(key string) error {
	return nil
}

func (c *FileStore) Increment(key string, delta uint64) (uint64, error) {
	return 0, ErrNotSupport
}

func (c *FileStore) Decrement(key string, delta uint64) (uint64, error) {
	return 0, ErrNotSupport
}

func (c *FileStore) Flush() error {
	return ErrNotSupport
}
