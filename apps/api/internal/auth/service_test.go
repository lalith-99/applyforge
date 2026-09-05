package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

type fakeUserStore struct {
	byID     map[uuid.UUID]users.User
	byEmail  map[string]uuid.UUID
	byGoogle map[string]uuid.UUID
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{
		byID:     map[uuid.UUID]users.User{},
		byEmail:  map[string]uuid.UUID{},
		byGoogle: map[string]uuid.UUID{},
	}
}

func (f *fakeUserStore) CreateWithPassword(_ context.Context, email, passwordHash string) (users.User, error) {
	u := users.User{ID: uuid.New(), Email: email, PasswordHash: &passwordHash, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.byID[u.ID] = u
	f.byEmail[email] = u.ID
	return u, nil
}

func (f *fakeUserStore) CreateWithGoogle(_ context.Context, email, googleID string) (users.User, error) {
	now := time.Now()
	u := users.User{ID: uuid.New(), Email: email, GoogleID: &googleID, EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now}
	f.byID[u.ID] = u
	f.byEmail[email] = u.ID
	f.byGoogle[googleID] = u.ID
	return u, nil
}

func (f *fakeUserStore) GetByEmail(_ context.Context, email string) (users.User, error) {
	id, ok := f.byEmail[email]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	return f.byID[id], nil
}

func (f *fakeUserStore) GetByGoogleID(_ context.Context, googleID string) (users.User, error) {
	id, ok := f.byGoogle[googleID]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	return f.byID[id], nil
}

func (f *fakeUserStore) GetByID(_ context.Context, id uuid.UUID) (users.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserStore) LinkGoogleAccount(_ context.Context, id uuid.UUID, googleID string) (users.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return users.User{}, users.ErrNotFound
	}
	u.GoogleID = &googleID
	f.byID[id] = u
	f.byGoogle[googleID] = id
	return u, nil
}

type fakeSessionStore struct {
	byHash map[string]Session
}

func newFakeSessionStore() *fakeSessionStore {
	return &fakeSessionStore{byHash: map[string]Session{}}
}

func (f *fakeSessionStore) create(_ context.Context, userID uuid.UUID, tokenHash, _, _ string) (Session, error) {
	s := Session{ID: uuid.New(), UserID: userID, ExpiresAt: time.Now().Add(SessionTTL)}
	f.byHash[tokenHash] = s
	return s, nil
}

func (f *fakeSessionStore) getByTokenHash(_ context.Context, tokenHash string) (Session, error) {
	s, ok := f.byHash[tokenHash]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	return s, nil
}

func (f *fakeSessionStore) deleteByTokenHash(_ context.Context, tokenHash string) error {
	delete(f.byHash, tokenHash)
	return nil
}

func newTestService() *Service {
	return newServiceForTest(newFakeUserStore(), newFakeSessionStore(), GoogleConfig{})
}

func TestService_SignUp_And_Login(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	issued, err := svc.SignUp(ctx, "person@example.com", "supersecret1", "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if issued.Token == "" {
		t.Fatalf("expected a session token")
	}

	_, err = svc.SignUp(ctx, "person@example.com", "anotherpassword", "test-agent", "127.0.0.1")
	if !errors.Is(err, ErrEmailInUse) {
		t.Fatalf("expected ErrEmailInUse, got %v", err)
	}

	loginIssued, err := svc.Login(ctx, "person@example.com", "supersecret1", "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if loginIssued.User.ID != issued.User.ID {
		t.Fatalf("expected same user id across signup/login")
	}

	if _, err := svc.Login(ctx, "person@example.com", "wrong-password", "test-agent", "127.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	if _, err := svc.Login(ctx, "nobody@example.com", "whatever1", "test-agent", "127.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for unknown email, got %v", err)
	}
}

func TestService_CurrentUser_And_Logout(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	issued, err := svc.SignUp(ctx, "session@example.com", "supersecret1", "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	u, err := svc.CurrentUser(ctx, issued.Token)
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	if u.ID != issued.User.ID {
		t.Fatalf("expected resolved user to match issued user")
	}

	if err := svc.Logout(ctx, issued.Token); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if _, err := svc.CurrentUser(ctx, issued.Token); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound after logout, got %v", err)
	}
}

func TestService_GoogleAuthURL_NotConfigured(t *testing.T) {
	svc := newTestService()
	if _, err := svc.GoogleAuthURL("state"); !errors.Is(err, ErrGoogleNotConfigured) {
		t.Fatalf("expected ErrGoogleNotConfigured, got %v", err)
	}
}
