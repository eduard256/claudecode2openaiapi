package tokens

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Token struct {
	Name      string    `json:"name"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	path   string
	mu     sync.Mutex
	tokens []Token
}

const prefix = "sk-cc2oa-"

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.tokens = nil
		return nil
	}
	if err != nil {
		return err
	}
	if len(b) == 0 {
		s.tokens = nil
		return nil
	}
	var data struct {
		Tokens []Token `json:"tokens"`
	}
	if err = json.Unmarshal(b, &data); err != nil {
		return err
	}
	s.tokens = data.Tokens
	return nil
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(struct {
		Tokens []Token `json:"tokens"`
	}{s.tokens}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err = os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Add generates a new token and persists it. If a token with the same name
// exists it returns an error.
func (s *Store) Add(name string) (Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.tokens {
		if t.Name == name {
			return Token{}, errors.New("tokens: name already exists: " + name)
		}
	}

	t := Token{
		Name:      name,
		Token:     generate(),
		CreatedAt: time.Now().UTC(),
	}
	s.tokens = append(s.tokens, t)
	if err := s.save(); err != nil {
		s.tokens = s.tokens[:len(s.tokens)-1]
		return Token{}, err
	}
	return t, nil
}

func (s *Store) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, t := range s.tokens {
		if t.Name == name {
			s.tokens = append(s.tokens[:i], s.tokens[i+1:]...)
			return s.save()
		}
	}
	return errors.New("tokens: not found: " + name)
}

func (s *Store) List() []Token {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Token, len(s.tokens))
	copy(out, s.tokens)
	return out
}

func (s *Store) Find(token string) (Token, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tokens {
		if t.Token == token {
			return t, true
		}
	}
	return Token{}, false
}

// generate creates a cryptographically random token in the form sk-cc2oa-<32hex>.
func generate() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return prefix + fmt.Sprintf("%x", b[:])
}
