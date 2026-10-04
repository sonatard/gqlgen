// Package models holds the Go types bound to the tablemode test schema. Both generated
// packages bind to them, so one set of resolvers serves both exec modes.
package models

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
)

type Node interface {
	IsNode()
	GetID() string
}

type Character interface {
	IsCharacter()
	IsNode()
	GetID() string
}

type SearchResult interface {
	IsSearchResult()
}

type Human struct {
	ID        string
	Name      string
	FriendIDs []string
	AppearsIn []Episode
	HeightM   float64
	Mass      *float64
	Secret    *string
	Tags      []*string
	Wallet    *Money
	nickname  string
}

func (*Human) IsNode()                {}
func (*Human) IsCharacter()           {}
func (*Human) IsSearchResult()        {}
func (h *Human) GetID() string        { return h.ID }
func (h *Human) SetNickname(n string) { h.nickname = n }

// Height is a method field with an argument.
func (h *Human) Height(unit LengthUnit) float64 {
	if unit == LengthUnitFoot {
		return h.HeightM * 3.28084
	}
	return h.HeightM
}

// Nickname is a method field of the (value, ok) shape.
func (h *Human) Nickname() (string, bool) {
	return h.nickname, h.nickname != ""
}

// Droid is bound as a value type, so interfaces see both Droid and *Droid.
type Droid struct {
	ID        string
	Name      string
	FriendIDs []string
	AppearsIn []Episode
	Function  string
	Serial    *int
}

func (Droid) IsNode()         {}
func (Droid) IsCharacter()    {}
func (Droid) IsSearchResult() {}
func (d Droid) GetID() string { return d.ID }

// PrimaryFunction takes a context, so the field resolves concurrently.
func (d Droid) PrimaryFunction(ctx context.Context) (string, error) {
	if d.Function == "" {
		return "", errors.New("droid has no function")
	}
	return d.Function, nil
}

type Starship struct {
	ID      string
	Name    string
	LengthM float64
	History [][]int
}

func (*Starship) IsSearchResult() {}

func (s *Starship) Length(unit LengthUnit) (float64, error) {
	if s.LengthM < 0 {
		return 0, errors.New("unknown length")
	}
	if unit == LengthUnitFoot {
		return s.LengthM * 3.28084, nil
	}
	return s.LengthM, nil
}

type Review struct {
	Stars      int
	Commentary *string
	Time       *time.Time
	Tags       []string
	Episode    *Episode
}

type NullChain struct {
	Required      *string
	Optional      *string
	Child         *NullChain
	RequiredChild *NullChain
}

// Query is the Go type of fields that return the query type.
type Query struct{}

type Summary struct {
	Label string
	Count int
	Extra map[string]any
}

type ReviewInput struct {
	Stars      int
	Commentary *string
	Time       *time.Time
	Tags       []string
	Nested     *NestedInput
	Note       *string
	Episode    *Episode
	Wallet     *Money
}

type NestedInput struct {
	Value   string
	Numbers []int
	Deeper  *NestedInput
}

type ResolvedInput struct {
	Label string
	Shout string
}

type OmittableInput struct {
	Value graphql.Omittable[*string]
	Count graphql.Omittable[*int]
}

// Money is a custom scalar that counts cents and is written as a decimal string.
type Money int64

func (m Money) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(fmt.Sprintf("%d.%02d", m/100, m%100)))
}

func (m *Money) UnmarshalGQL(v any) error {
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("money must be a string, got %T", v)
	}
	whole, frac, _ := strings.Cut(s, ".")
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid money %q", s)
	}
	f := int64(0)
	if frac != "" {
		if f, err = strconv.ParseInt(frac, 10, 64); err != nil || len(frac) != 2 {
			return fmt.Errorf("invalid money %q", s)
		}
	}
	*m = Money(w*100 + f)
	return nil
}

type Role string

const (
	RoleAdmin Role = "ADMIN"
	RoleUser  Role = "USER"
	RoleGuest Role = "GUEST"
)

func (e Role) IsValid() bool {
	switch e {
	case RoleAdmin, RoleUser, RoleGuest:
		return true
	}
	return false
}

func (e *Role) UnmarshalGQL(v any) error { return unmarshalEnum(e, v, "Role") }
func (e Role) MarshalGQL(w io.Writer)    { _, _ = io.WriteString(w, strconv.Quote(string(e))) }

type Episode string

const (
	EpisodeNewhope Episode = "NEWHOPE"
	EpisodeEmpire  Episode = "EMPIRE"
	EpisodeJedi    Episode = "JEDI"
)

func (e Episode) IsValid() bool {
	switch e {
	case EpisodeNewhope, EpisodeEmpire, EpisodeJedi:
		return true
	}
	return false
}

func (e *Episode) UnmarshalGQL(v any) error { return unmarshalEnum(e, v, "Episode") }
func (e Episode) MarshalGQL(w io.Writer)    { _, _ = io.WriteString(w, strconv.Quote(string(e))) }

type LengthUnit string

const (
	LengthUnitMeter LengthUnit = "METER"
	LengthUnitFoot  LengthUnit = "FOOT"
)

func (e LengthUnit) IsValid() bool { return e == LengthUnitMeter || e == LengthUnitFoot }

func (e *LengthUnit) UnmarshalGQL(v any) error { return unmarshalEnum(e, v, "LengthUnit") }

func (e LengthUnit) MarshalGQL(
	w io.Writer,
) {
	_, _ = io.WriteString(w, strconv.Quote(string(e)))
}

type enum interface {
	~string
	IsValid() bool
}

func unmarshalEnum[E enum](e *E, v any, name string) error {
	s, ok := v.(string)
	if !ok {
		return errors.New("enums must be strings")
	}
	*e = E(s)
	if !(*e).IsValid() {
		return fmt.Errorf("%s is not a valid %s", s, name)
	}
	return nil
}
