package cache

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
)

const (
	DEFAULT              = time.Duration(0)
	FOREVER              = time.Duration(-1)
	CACHE_MIDDLEWARE_KEY = "gincontrib.cache"
)

var (
	PageCachePrefix = "io.steeve.pulsar.page.cache"
	ErrCacheMiss    = errors.New("cache: key not found.")
	ErrNotStored    = errors.New("cache: not stored.")
	ErrNotSupport   = errors.New("cache: not support.")
	log             = logging.MustGetLogger("btplayer")
)

type CacheStore interface {
	Get(key string, value interface{}) error
	Set(key string, value interface{}, expire time.Duration) error
	Add(key string, value interface{}, expire time.Duration) error
	Replace(key string, data interface{}, expire time.Duration) error
	Delete(key string) error
	Increment(key string, data uint64) (uint64, error)
	Decrement(key string, data uint64) (uint64, error)
	Flush() error
}

type responseCache struct {
	Status int
	Header http.Header
	Data   []byte
}

type cachedWriter struct {
	gin.ResponseWriter
	status  int
	written bool
	store   CacheStore
	expire  time.Duration
	key     string
}

func cacheKey(prefix string, u string) string {
	h := sha1.New()
	io.WriteString(h, u)
	return prefix + ":" + hex.EncodeToString(h.Sum(nil))
}

func newCachedWriter(store CacheStore, expire time.Duration, writer gin.ResponseWriter, key string) *cachedWriter {
	return &cachedWriter{writer, 0, false, store, expire, key}
}

func (w *cachedWriter) WriteHeader(code int) {
	w.status = code
	w.written = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *cachedWriter) Status() int {
	return w.status
}

func (w *cachedWriter) Written() bool {
	return w.written
}

func (w *cachedWriter) Write(data []byte) (int, error) {
	ret, err := w.ResponseWriter.Write(data)
	if err == nil {
		//cache response
		store := w.store
		val := responseCache{
			w.status,
			w.Header(),
			data,
		}
		err = store.Set(w.key, val, w.expire)
		if err != nil {
			// need logger
		}
	}
	return ret, err
}

// Cache Middleware
func Cache(store CacheStore, expire time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var cache responseCache
		key := cacheKey(PageCachePrefix, ctx.Request.URL.RequestURI())
		if err := store.Get(key, &cache); err == nil {
			for k, vals := range cache.Header {
				for _, v := range vals {
					ctx.Writer.Header().Add(k, v)
				}
			}
			ctx.Abort(cache.Status)
			ctx.Writer.Write(cache.Data)
		} else {
			// replace writer
			writer := ctx.Writer
			ctx.Writer = newCachedWriter(store, expire, ctx.Writer, key)
			ctx.Next()
			ctx.Writer = writer
		}
	}
}
