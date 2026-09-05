package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	"github.com/lalithlochan/applyforge/apps/api/internal/users"
)

// ErrInvalidCredentials is returned for unknown emails or wrong passwords.
var ErrInvalidCredentials = errors.New("invalid email or password")

// ErrEmailInUse is returned when signing up with an email already registered.
var ErrEmailInUse = errors.New("email already in use")

// UserStore is the subset of the users repository the auth service depends on.
type UserStore interface {
	CreateWithPassword(ctx context.Context, email, passwordHash string) (users.User, error)
	CreateWithGoogle(ctx context.Context, email, googleID string) (users.User, error)
	GetByEmail(ctx context.Context, email string) (users.User, error)
	GetByGoogleID(ctx context.Context, googleID string) (users.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (users.User, error)
	LinkGoogleAccount(ctx context.Context, id uuid.UUID, googleID string) (users.User, error)
}

// SessionStore is the subset of session persistence the auth service depends
// on. Kept as an interface (rather than the concrete *sessionRepository) so
// Service can be unit tested with a fake, without a real database.
type SessionStore interface {
	create(ctx context.Context, userID uuid.UUID, tokenHash, userAgent, ip string) (Session, error)
	getByTokenHash(ctx context.Context, tokenHash string) (Session, error)
	deleteByTokenHash(ctx context.Context, tokenHash string) error
}

// Issued represents a freshly created session and the raw token to hand to
// the client (only the hash is persisted server-side).
type Issued struct {
	User  users.User
	Token string
}

// Service implements the email/password and Google OAuth flows.
type Service struct {
	users    UserStore
	sessions SessionStore
	google   GoogleConfig
}

// NewService builds an auth Service backed by a real database pool.
func NewService(pool *database.Pool, userStore UserStore, google GoogleConfig) *Service {
	return &Service{
		users:    userStore,
		sessions: newSessionRepository(pool),
		google:   google,
	}
}

// newServiceForTest builds a Service from fakes, bypassing the database.
func newServiceForTest(userStore UserStore, sessionStore SessionStore, google GoogleConfig) *Service {
	return &Service{users: userStore, sessions: sessionStore, google: google}
}

// SignUp creates a new email/password account and an initial session.
func (s *Service) SignUp(ctx context.Context, email, password, userAgent, ip string) (Issued, error) {
	if _, err := s.users.GetByEmail(ctx, email); err == nil {
		return Issued{}, ErrEmailInUse
	} else if !errors.Is(err, users.ErrNotFound) {
		return Issued{}, err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return Issued{}, err
	}

	u, err := s.users.CreateWithPassword(ctx, email, hash)
	if err != nil {
		return Issued{}, err
	}

	return s.issueSession(ctx, u, userAgent, ip)
}

// Login validates email/password credentials and issues a new session.
func (s *Service) Login(ctx context.Context, email, password, userAgent, ip string) (Issued, error) {
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return Issued{}, ErrInvalidCredentials
		}
		return Issued{}, err
	}

	if u.PasswordHash == nil || !VerifyPassword(*u.PasswordHash, password) {
		return Issued{}, ErrInvalidCredentials
	}

	return s.issueSession(ctx, u, userAgent, ip)
}

// Logout invalidates the session identified by the raw token.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.sessions.deleteByTokenHash(ctx, hashToken(token))
}

// CurrentUser resolves the raw session token into the authenticated user.
func (s *Service) CurrentUser(ctx context.Context, token string) (users.User, error) {
	session, err := s.sessions.getByTokenHash(ctx, hashToken(token))
	if err != nil {
		return users.User{}, err
	}
	return s.users.GetByID(ctx, session.UserID)
}

// GoogleAuthURL returns the Google consent screen URL for the given CSRF state.
func (s *Service) GoogleAuthURL(state string) (string, error) {
	return s.google.AuthURL(state)
}

// CompleteGoogleLogin exchanges an authorization code, finds-or-creates the
// corresponding user, and issues a new session.
func (s *Service) CompleteGoogleLogin(ctx context.Context, code, userAgent, ip string) (Issued, error) {
	info, err := s.google.Exchange(ctx, code)
	if err != nil {
		return Issued{}, err
	}

	u, err := s.users.GetByGoogleID(ctx, info.Sub)
	switch {
	case err == nil:
		// existing Google-linked user
	case errors.Is(err, users.ErrNotFound):
		existing, emailErr := s.users.GetByEmail(ctx, info.Email)
		switch {
		case emailErr == nil:
			u, err = s.users.LinkGoogleAccount(ctx, existing.ID, info.Sub)
			if err != nil {
				return Issued{}, err
			}
		case errors.Is(emailErr, users.ErrNotFound):
			u, err = s.users.CreateWithGoogle(ctx, info.Email, info.Sub)
			if err != nil {
				return Issued{}, err
			}
		default:
			return Issued{}, emailErr
		}
	default:
		return Issued{}, err
	}

	return s.issueSession(ctx, u, userAgent, ip)
}

func (s *Service) issueSession(ctx context.Context, u users.User, userAgent, ip string) (Issued, error) {
	token, err := newSessionToken()
	if err != nil {
		return Issued{}, err
	}

	if _, err := s.sessions.create(ctx, u.ID, hashToken(token), userAgent, ip); err != nil {
		return Issued{}, err
	}

	return Issued{User: u, Token: token}, nil
}
