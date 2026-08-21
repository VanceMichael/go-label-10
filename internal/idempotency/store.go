package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/VanceMichael/go-base-airbridge/internal/domain"
)

type Record struct {
	TenantID    string
	Key         string
	Fingerprint string
	Status      int
	Body        []byte
	CreatedAt   time.Time
	ExpiresAt   time.Time
}
type Store struct {
	mu      sync.Mutex
	records map[string]*execution
}

func New() *Store { return &Store{records: map[string]*execution{}} }

func Fingerprint(method, path string, body []byte) string {
	sum := sha256.Sum256(append([]byte(method+"\n"+path+"\n"), body...))
	return hex.EncodeToString(sum[:])
}

func (s *Store) Begin(tenant, key, fingerprint string, now time.Time) (Record, bool, error) {
	return s.BeginContext(context.Background(), tenant, key, fingerprint, now)
}

func (s *Store) BeginContext(ctx context.Context, tenant, key, fingerprint string, now time.Time) (Record, bool, error) {
	if tenant == "" || key == "" || fingerprint == "" {
		return Record{}, false, domain.ErrInvalid
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	compound := tenant + "|" + key
	if old, ok := s.records[compound]; ok && now.Before(old.record.ExpiresAt) {
		if old.record.Fingerprint != fingerprint {
			return Record{}, false, domain.ErrConflict
		}
		return old.snapshot(), true, nil
	}
	v := newExecution(Record{TenantID: tenant, Key: key, Fingerprint: fingerprint, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour), Status: 102})
	s.records[compound] = v
	return v.snapshot(), false, nil
}

func (s *Store) Complete(tenant, key string, status int, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	compound := tenant + "|" + key
	v, ok := s.records[compound]
	if !ok {
		return domain.ErrNotFound
	}
	return v.complete(status, body)
}

func (s *Store) Abort(tenant, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	compound := tenant + "|" + key
	v, ok := s.records[compound]
	if !ok {
		return domain.ErrNotFound
	}
	delete(s.records, compound)
	v.abort()
	return nil
}

func (s *Store) Cleanup(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for k, v := range s.records {
		if !now.Before(v.record.ExpiresAt) {
			delete(s.records, k)
			v.expire()
			n++
		}
	}
	return n
}
