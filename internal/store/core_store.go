package store

// CoreStore is the minimal persistence root used by provider-free novel-core.
// It deliberately does not initialize or expose any legacy pipeline substore.
type CoreStore struct {
	dir string
	io  *IO
}

func NewCoreStore(dir string) *CoreStore {
	return &CoreStore{dir: dir, io: newIO(dir)}
}

func (s *CoreStore) Dir() string { return s.dir }
