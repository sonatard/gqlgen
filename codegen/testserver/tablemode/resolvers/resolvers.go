// Package resolvers implements the tablemode test schema once for both generated
// packages. The generated resolver interfaces of the two packages have the same methods,
// so the types here satisfy both.
package resolvers

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/99designs/gqlgen/codegen/testserver/tablemode/models"
	"github.com/99designs/gqlgen/graphql"
)

var (
	humans = map[string]*models.Human{
		"1000": {
			ID: "1000", Name: "Luke", FriendIDs: []string{"1002", "2000", "2001"},
			AppearsIn: []models.Episode{models.EpisodeNewhope, models.EpisodeEmpire},
			HeightM:   1.72, Mass: new(77.0), Secret: new("father"),
			Tags: []*string{new("jedi"), nil}, Wallet: new(models.Money(1234)), Code: "luke",
		},
		"1001": {
			ID: "1001", Name: "Vader", AppearsIn: []models.Episode{models.EpisodeEmpire},
			HeightM: 2.02,
		},
		"1002": {
			ID: "1002", Name: "Han", FriendIDs: []string{"1000"},
			AppearsIn: []models.Episode{models.EpisodeJedi}, HeightM: 1.8,
		},
	}
	droids = map[string]models.Droid{
		"2000": {
			ID:        "2000",
			Name:      "C-3PO",
			Function:  "Protocol",
			FriendIDs: []string{"1000"},
			Serial:    new(3),
		},
		"2001": {ID: "2001", Name: "R2-D2", FriendIDs: []string{"2000"}},
	}
	starships = map[string]*models.Starship{
		"3000": {ID: "3000", Name: "Falcon", LengthM: 34.37, History: [][]int{{1, 2}, {3}}},
		"3001": {ID: "3001", Name: "X-Wing", LengthM: -1, History: [][]int{}},
	}
)

func init() {
	humans["1002"].SetNickname("Solo")
}

func character(id string) models.Character {
	if h, ok := humans[id]; ok {
		return h
	}
	if d, ok := droids[id]; ok {
		// A value Droid exercises the non-pointer case of the interface marshalers.
		return d
	}
	return nil
}

func friends(ids []string) []models.Character {
	res := make([]models.Character, 0, len(ids))
	for _, id := range ids {
		res = append(res, character(id))
	}
	return res
}

type Query struct{}

func (Query) Hero(ctx context.Context, episode *models.Episode) (models.Character, error) {
	if episode != nil && *episode == models.EpisodeEmpire {
		return humans["1000"], nil
	}
	if episode != nil && *episode == models.EpisodeNewhope {
		return nil, nil
	}
	return droids["2001"], nil
}

func (Query) Human(ctx context.Context, id string) (*models.Human, error) {
	return humans[id], nil
}

func (Query) Droid(ctx context.Context, id string) (*models.Droid, error) {
	d, ok := droids[id]
	if !ok {
		return nil, fmt.Errorf("no droid %s", id)
	}
	return &d, nil
}

func (Query) Node(ctx context.Context, id string) (models.Node, error) {
	if c := character(id); c != nil {
		return c, nil
	}
	return nil, nil
}

func (Query) Search(ctx context.Context, text string) ([]models.SearchResult, error) {
	var res []models.SearchResult
	ids := make([]string, 0, len(humans)+len(droids)+len(starships))
	for id := range humans {
		ids = append(ids, id)
	}
	for id := range droids {
		ids = append(ids, id)
	}
	for id := range starships {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		switch {
		case humans[id] != nil && strings.Contains(humans[id].Name, text):
			res = append(res, humans[id])
		case droids[id].ID != "" && strings.Contains(droids[id].Name, text):
			d := droids[id]
			res = append(res, &d)
		case starships[id] != nil && strings.Contains(starships[id].Name, text):
			res = append(res, starships[id])
		}
	}
	return res, nil
}

func (Query) Starship(ctx context.Context, id string) (*models.Starship, error) {
	return starships[id], nil
}

func (Query) Reviews(
	ctx context.Context,
	episode models.Episode,
	since *time.Time,
) ([]*models.Review, error) {
	t := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	reviews := []*models.Review{
		{Stars: 5, Commentary: new("great"), Time: &t, Tags: []string{"a"}, Episode: &episode},
		{Stars: 2, Tags: []string{}},
	}
	if since != nil && since.After(t) {
		return reviews[1:], nil
	}
	return reviews, nil
}

func review(in models.ReviewInput) *models.Review {
	r := &models.Review{
		Stars:      in.Stars,
		Commentary: in.Commentary,
		Time:       in.Time,
		Tags:       in.Tags,
		Episode:    in.Episode,
		Code:       in.Code,
	}
	if r.Tags == nil {
		r.Tags = []string{}
	}
	if in.Nested != nil {
		for n := in.Nested; n != nil; n = n.Deeper {
			r.Tags = append(r.Tags, n.Value)
			for _, i := range n.Numbers {
				r.Tags = append(r.Tags, strconv.Itoa(i))
			}
		}
	}
	if in.Note != nil {
		r.Tags = append(r.Tags, "note:"+*in.Note)
	}
	if in.Wallet != nil {
		r.Tags = append(r.Tags, fmt.Sprintf("wallet:%d", *in.Wallet))
	}
	return r
}

func (Query) EchoReview(ctx context.Context, input models.ReviewInput) (*models.Review, error) {
	return review(input), nil
}

func (Query) EchoReviews(
	ctx context.Context,
	inputs []*models.ReviewInput,
) ([]*models.Review, error) {
	if inputs == nil {
		return nil, nil
	}
	res := make([]*models.Review, 0, len(inputs))
	for _, in := range inputs {
		res = append(res, review(*in))
	}
	return res, nil
}

func (Query) EchoMap(ctx context.Context, input map[string]any) (map[string]any, error) {
	if input == nil {
		return map[string]any{"empty": true}, nil
	}
	res := map[string]any{}
	for k, v := range input {
		res[k] = describe(v)
	}
	return res, nil
}

func describe(v any) string {
	switch v := v.(type) {
	case *string:
		if v != nil {
			return "*string:" + *v
		}
	case *int:
		if v != nil {
			return fmt.Sprintf("*int:%d", *v)
		}
	case *models.Role:
		if v != nil {
			return "*Role:" + string(*v)
		}
	}
	return fmt.Sprintf("%T:%v", v, v)
}

func (Query) EchoResolved(ctx context.Context, input models.ResolvedInput) (string, error) {
	return input.Label + "/" + input.Shout, nil
}

func (Query) EchoOmittable(ctx context.Context, input models.OmittableInput) (string, error) {
	describe := func(set bool, v any) string {
		if !set {
			return "unset"
		}
		return fmt.Sprintf("%v", v)
	}
	value := "nil"
	if v := input.Value.Value(); v != nil {
		value = *v
	}
	count := "nil"
	if c := input.Count.Value(); c != nil {
		count = strconv.Itoa(*c)
	}
	return describe(input.Value.IsSet(), value) + "," + describe(input.Count.IsSet(), count), nil
}

func (Query) EchoNullified(ctx context.Context, value *string) (string, error) {
	if value == nil {
		return "nil", nil
	}
	return *value, nil
}

func (Query) Money(ctx context.Context, amount models.Money) (models.Money, error) {
	return amount * 2, nil
}

func (Query) Roles(ctx context.Context) ([]models.Role, error) {
	return []models.Role{models.RoleAdmin, models.RoleGuest}, nil
}

func (Query) WithDefaults(
	ctx context.Context,
	a *int,
	b *string,
	c []models.Episode,
	d *models.NestedInput,
) (string, error) {
	parts := []string{}
	if a != nil {
		parts = append(parts, strconv.Itoa(*a))
	}
	if b != nil {
		parts = append(parts, *b)
	}
	parts = append(parts, fmt.Sprint(c))
	if d != nil {
		parts = append(parts, d.Value, fmt.Sprint(d.Numbers))
	}
	return strings.Join(parts, "|"), nil
}

func (Query) NullChain(ctx context.Context) (*models.NullChain, error) {
	return &models.NullChain{
		Required: new("root"),
		Child: &models.NullChain{
			Optional:      new("child"),
			RequiredChild: &models.NullChain{Required: new("grandchild")},
		},
	}, nil
}

func (Query) Failing(ctx context.Context) (string, error) {
	return "", errors.New("failing on purpose")
}

func (Query) FailingOptional(ctx context.Context) (*string, error) {
	return nil, errors.New("optional failure")
}

func (Query) Panicking(ctx context.Context) (*string, error) {
	panic("panicking on purpose")
}

func (Query) Summary(ctx context.Context) (*models.Summary, error) {
	return &models.Summary{Label: "summary", Count: 3, Extra: map[string]any{"k": "v"}}, nil
}

func (Query) SecretNumber(ctx context.Context) (*int, error) {
	return new(42), nil
}

type Mutation struct{}

func (Mutation) CreateReview(
	ctx context.Context,
	episode models.Episode,
	input models.ReviewInput,
) (*models.Review, error) {
	r := review(input)
	r.Episode = &episode
	return r, nil
}

func (Mutation) SetRole(ctx context.Context, role models.Role) (models.Role, error) {
	return role, nil
}

type Human struct{}

func (Human) Friends(ctx context.Context, obj *models.Human) ([]models.Character, error) {
	return friends(obj.FriendIDs), nil
}

func (Human) Starships(ctx context.Context, obj *models.Human) ([]*models.Starship, error) {
	if obj.ID == "1001" {
		return nil, errors.New("starships are classified")
	}
	return []*models.Starship{starships["3000"], starships["3001"]}, nil
}

type Droid struct{}

func (Droid) Friends(ctx context.Context, obj *models.Droid) ([]models.Character, error) {
	return friends(obj.FriendIDs), nil
}

type ResolvedInput struct{}

func (ResolvedInput) Shout(ctx context.Context, obj *models.ResolvedInput, data *string) error {
	if data == nil {
		obj.Shout = "quiet"
		return nil
	}
	if *data == "" {
		return errors.New("shout must not be empty")
	}
	obj.Shout = strings.ToUpper(*data)
	return nil
}

func Upper(ctx context.Context, obj any, next graphql.Resolver) (any, error) {
	res, err := next(ctx)
	if s, ok := res.(string); ok {
		return strings.ToUpper(s), err
	}
	return res, err
}

func Auth(ctx context.Context, obj any, next graphql.Resolver, role models.Role) (any, error) {
	if role == models.RoleAdmin {
		return nil, fmt.Errorf("role %s required for %T", role, obj)
	}
	return next(ctx)
}

func Length(
	ctx context.Context,
	obj any,
	next graphql.Resolver,
	minimum int,
	maximum *int,
) (any, error) {
	res, err := next(ctx)
	if err != nil {
		return res, err
	}
	var s string
	switch v := res.(type) {
	case string:
		s = v
	case *string:
		if v == nil {
			return res, nil
		}
		s = *v
	default:
		return nil, fmt.Errorf("length cannot check %T", res)
	}
	if len(s) < minimum || (maximum != nil && len(s) > *maximum) {
		return nil, fmt.Errorf("length of %q is out of range", s)
	}
	return res, nil
}

func Nullify(ctx context.Context, obj any, next graphql.Resolver) (any, error) {
	if _, err := next(ctx); err != nil {
		return nil, err
	}
	return nil, nil
}

func ValidReview(ctx context.Context, obj any, next graphql.Resolver) (any, error) {
	res, err := next(ctx)
	if err != nil {
		return res, err
	}
	var stars int
	switch v := res.(type) {
	case models.ReviewInput:
		stars = v.Stars
		v.Tags = append(v.Tags, "validated")
		res = v
	case *models.ReviewInput:
		stars = v.Stars
		v.Tags = append(v.Tags, "validated")
	}
	if stars > 5 {
		return nil, errors.New("a review has at most 5 stars")
	}
	return res, nil
}

func Trace(ctx context.Context, obj any, next graphql.Resolver, label *string) (any, error) {
	res, err := next(ctx)
	if s, ok := res.(string); ok && label != nil {
		return s + " [" + *label + "]", err
	}
	return res, err
}

func LogQuery(ctx context.Context, obj any, next graphql.Resolver) (any, error) {
	return next(ctx)
}

func LogMutation(ctx context.Context, obj any, next graphql.Resolver) (any, error) {
	return next(ctx)
}

// HeightComplexity makes heights in feet expensive, to test that complexity sees the
// arguments.
func HeightComplexity(childComplexity int, unit models.LengthUnit) int {
	if unit == models.LengthUnitFoot {
		return 100
	}
	return 1
}
