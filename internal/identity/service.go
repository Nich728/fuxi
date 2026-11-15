// Package identity owns users and devices (SYSTEM_DESIGN §4.1).
//
// Sign-in is delegated to Cloudflare Access (ADR-0018): this package verifies
// the token Access attaches to each request, creates the user on first sight
// and puts a principal in the request context.
package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Nich728/fuxi/internal/identity/identitydb"
	"github.com/Nich728/fuxi/internal/platform/clock"
	"github.com/Nich728/fuxi/internal/platform/principal"
)

// Errors returned by Service.
var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid input")
)

// Service manages users and devices.
type Service struct {
	q          *identitydb.Queries
	clock      clock.Clock
	ownerEmail string
}

// NewService returns a Service. ownerEmail, if set, gets the owner role when
// its account is first created.
func NewService(db identitydb.DBTX, clk clock.Clock, ownerEmail string) *Service {
	return &Service{q: identitydb.New(db), clock: clk, ownerEmail: strings.ToLower(strings.TrimSpace(ownerEmail))}
}

// User is the account as the API shows it.
type User struct {
	ID          uuid.UUID       `json:"id"`
	Email       string          `json:"email"`
	Role        string          `json:"role"`
	Timezone    string          `json:"timezone"`
	DayStartsAt int             `json:"day_starts_at"`
	Settings    json.RawMessage `json:"settings"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Resolve returns the principal for a verified email, creating the user on
// their first request.
func (s *Service) Resolve(ctx context.Context, email string) (principal.Principal, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := mail.ParseAddress(email); err != nil {
		return principal.Principal{}, fmt.Errorf("%w: email", ErrInvalid)
	}
	now := s.clock.Now()
	u, err := s.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		id, idErr := uuid.NewV7()
		if idErr != nil {
			return principal.Principal{}, idErr
		}
		role := "member"
		if email == s.ownerEmail {
			role = "owner"
		}
		if insErr := s.q.InsertUser(ctx, identitydb.InsertUserParams{ID: id, Email: email, Role: role, CreatedAt: now}); insErr != nil {
			return principal.Principal{}, fmt.Errorf("create user: %w", insErr)
		}
		// Read back: a concurrent first request may have created the row instead.
		u, err = s.q.GetUserByEmail(ctx, email)
	}
	if err != nil {
		return principal.Principal{}, fmt.Errorf("load user: %w", err)
	}
	if err := s.q.TouchUser(ctx, identitydb.TouchUserParams{ID: u.ID, Now: now}); err != nil {
		return principal.Principal{}, fmt.Errorf("touch user: %w", err)
	}
	return toPrincipal(u), nil
}

func toPrincipal(u identitydb.User) principal.Principal {
	return principal.Principal{
		UserID: u.ID, Email: u.Email, Role: u.Role,
		Timezone: u.Timezone, DayStartsAt: int(u.DayStartsAt),
	}
}

// Me returns the user's account.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (User, error) {
	u, err := s.q.GetUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("load user: %w", err)
	}
	return toUser(u), nil
}

func toUser(u identitydb.User) User {
	return User{
		ID: u.ID, Email: u.Email, Role: u.Role, Timezone: u.Timezone,
		DayStartsAt: int(u.DayStartsAt), Settings: u.Settings, CreatedAt: u.CreatedAt,
	}
}

// SettingsPatch changes account settings; nil fields are left alone.
type SettingsPatch struct {
	ExplanationLanguage *string `json:"explanation_language"` // en | id (PRD C1)
	ToneColours         *bool   `json:"tone_colours"`         // PRD C2
	Timezone            *string `json:"timezone"`             // IANA name
	DayStartsAt         *int    `json:"day_starts_at"`        // 0–23 (PRD R5)
}

// UpdateSettings applies p.
func (s *Service) UpdateSettings(ctx context.Context, userID uuid.UUID, p SettingsPatch) (User, error) {
	settings := map[string]any{}
	if p.ExplanationLanguage != nil {
		if *p.ExplanationLanguage != "en" && *p.ExplanationLanguage != "id" {
			return User{}, fmt.Errorf("%w: explanation_language must be en or id", ErrInvalid)
		}
		settings["explanation_language"] = *p.ExplanationLanguage
	}
	if p.ToneColours != nil {
		settings["tone_colours"] = *p.ToneColours
	}
	if p.Timezone != nil {
		if *p.Timezone == "" || *p.Timezone == "Local" {
			return User{}, fmt.Errorf("%w: timezone", ErrInvalid)
		}
		if _, err := time.LoadLocation(*p.Timezone); err != nil {
			return User{}, fmt.Errorf("%w: unknown timezone %q", ErrInvalid, *p.Timezone)
		}
	}
	var dayStart *int16
	if p.DayStartsAt != nil {
		if *p.DayStartsAt < 0 || *p.DayStartsAt > 23 {
			return User{}, fmt.Errorf("%w: day_starts_at must be 0–23", ErrInvalid)
		}
		v := int16(*p.DayStartsAt)
		dayStart = &v
	}
	b, err := json.Marshal(settings)
	if err != nil {
		return User{}, err
	}
	u, err := s.q.UpdateUserSettings(ctx, identitydb.UpdateUserSettingsParams{
		ID: userID, Settings: b, Timezone: p.Timezone, DayStartsAt: dayStart,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("update settings: %w", err)
	}
	return toUser(u), nil
}

// Device is a browser or installed PWA the user studies on.
type Device struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// RegisterDevice creates or renames the device. The client generates the id
// once and keeps it, so this is safe to call on every app start.
func (s *Service) RegisterDevice(ctx context.Context, userID, deviceID uuid.UUID, name string) (Device, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return Device{}, fmt.Errorf("%w: name must be 1–100 characters", ErrInvalid)
	}
	d, err := s.q.UpsertDevice(ctx, identitydb.UpsertDeviceParams{
		ID: deviceID, UserID: userID, Name: name, Now: s.clock.Now(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// The id belongs to someone else's device.
		return Device{}, ErrNotFound
	}
	if err != nil {
		return Device{}, fmt.Errorf("register device: %w", err)
	}
	return Device{ID: d.ID, Name: d.Name, LastSeenAt: d.LastSeenAt, CreatedAt: d.CreatedAt}, nil
}

// Devices lists the user's devices.
func (s *Service) Devices(ctx context.Context, userID uuid.UUID) ([]Device, error) {
	rows, err := s.q.ListDevices(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	out := make([]Device, len(rows))
	for i, d := range rows {
		out[i] = Device{ID: d.ID, Name: d.Name, LastSeenAt: d.LastSeenAt, CreatedAt: d.CreatedAt}
	}
	return out, nil
}

// OwnsDevice reports whether deviceID is one of the user's devices.
func (s *Service) OwnsDevice(ctx context.Context, userID, deviceID uuid.UUID) (bool, error) {
	ok, err := s.q.DeviceExists(ctx, identitydb.DeviceExistsParams{ID: deviceID, UserID: userID})
	if err != nil {
		return false, fmt.Errorf("check device: %w", err)
	}
	return ok, nil
}

// RemoveDevice forgets a device. Signing the browser out is Cloudflare's job
// (ADR-0018); the client clears its local data when it sees its device gone.
func (s *Service) RemoveDevice(ctx context.Context, userID, deviceID uuid.UUID) error {
	n, err := s.q.DeleteDevice(ctx, identitydb.DeleteDeviceParams{ID: deviceID, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
