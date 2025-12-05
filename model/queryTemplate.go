package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ermes-thesis/ermes-common/utils"
)

type QueryTemplate struct {
	Select string            `json:"SELECT"`
	As     string            `json:"AS"`
	Where  *ConditionWrapper `json:"WHERE,omitempty"`
}

var allowedSelectValues = map[string]struct{}{
	"ALL":    {},
	"UNIQUE": {},
	"MAX":    {},
	"MIN":    {},
}

func (q *QueryTemplate) Validate() error {
	if strings.TrimSpace(q.Select) == "" {
		return errors.New("SELECT field is required")
	}

	_, allowed := allowedSelectValues[q.Select]
	if !allowed {
		return errors.New("invalid select value: " + q.Select)
	}

	if strings.TrimSpace(q.As) == "" {
		return errors.New("AS field is required")
	}

	if q.Where != nil {
		if err := q.Where.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (q *QueryTemplate) ToRedisSearchCommand() (string, error) {
	if err := q.Validate(); err != nil {
		return "", err
	}

	query := "*"
	if q.Where != nil && q.Where.Condition != nil {
		rq, err := q.Where.Condition.ToRedisQuery()
		if err != nil {
			return "", err
		}
		query = rq
	}

	return query, nil
}

func (q *QueryTemplate) ReplacePlaceholder(vars map[string]any) (*QueryTemplate, error) {
	if q.Where != nil && q.Where.Condition != nil {
		cond, err := q.Where.Condition.ReplacePlaceholder(vars)
		return &QueryTemplate{
			Select: q.Select,
			As:     q.As,
			Where:  &ConditionWrapper{Condition: cond},
		}, err
	}
	return q, nil
}

func (q *QueryTemplate) ReplacePlaceholderFromStrings(vars map[string]string) (*QueryTemplate, error) {
	anyVars := make(map[string]any, len(vars))
	for k, v := range vars {
		anyVars[k] = v
	}
	return q.ReplacePlaceholder(anyVars)
}

type Condition interface {
	Validate() error
	ToRedisQuery() (string, error)
	ReplacePlaceholder(vars map[string]any) (Condition, error)
}

type AndCondition struct {
	Conditions []Condition `json:"AND"`
}

func (a *AndCondition) Validate() error {
	if len(a.Conditions) == 0 {
		return errors.New("AND condition requires at least one sub-condition")
	}
	for _, c := range a.Conditions {
		if err := c.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (a *AndCondition) ToRedisQuery() (string, error) {
	parts := make([]string, 0, len(a.Conditions))
	for _, c := range a.Conditions {
		part, err := c.ToRedisQuery()
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " "), nil
}

func (a *AndCondition) ReplacePlaceholder(vars map[string]any) (Condition, error) {
	andCond := &AndCondition{}
	for _, c := range a.Conditions {
		cond, err := c.ReplacePlaceholder(vars)
		if err != nil {
			return nil, err
		}
		andCond.Conditions = append(andCond.Conditions, cond)
	}
	return andCond, nil
}

func (a *AndCondition) String() string {
	return fmt.Sprintf("AndCondition{Conditions:%+v}", a.Conditions)
}

type OrCondition struct {
	Conditions []Condition `json:"OR"`
}

func (o *OrCondition) Validate() error {
	if len(o.Conditions) == 0 {
		return errors.New("OR condition requires at least one sub-condition")
	}
	for _, c := range o.Conditions {
		if err := c.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (o *OrCondition) ToRedisQuery() (string, error) {
	parts := make([]string, 0, len(o.Conditions))
	for _, c := range o.Conditions {
		part, err := c.ToRedisQuery()
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("(%s)", part))
	}
	return fmt.Sprintf("(%s)", strings.Join(parts, " | ")), nil
}

func (o *OrCondition) ReplacePlaceholder(vars map[string]any) (Condition, error) {
	orCond := &OrCondition{}
	for _, c := range o.Conditions {
		cond, err := c.ReplacePlaceholder(vars)
		if err != nil {
			return nil, err
		}
		orCond.Conditions = append(orCond.Conditions, cond)
	}
	return orCond, nil
}

func (o *OrCondition) String() string {
	return fmt.Sprintf("OrCondition{Conditions:%+v}", o.Conditions)
}

type NotCondition struct {
	Condition Condition `json:"NOT"`
}

func (n *NotCondition) Validate() error {
	if n.Condition == nil {
		return errors.New("NOT condition requires a sub-condition")
	}
	return n.Condition.Validate()
}

func (n *NotCondition) ToRedisQuery() (string, error) {
	part, err := n.Condition.ToRedisQuery()
	if err != nil {
		return "", err
	}
	return "-" + part, nil
}

func (n *NotCondition) ReplacePlaceholder(vars map[string]any) (Condition, error) {
	if n.Condition != nil {
		cond, err := n.Condition.ReplacePlaceholder(vars)
		return &NotCondition{Condition: cond}, err
	}
	return n, nil
}

func (n *NotCondition) String() string {
	return fmt.Sprintf("NotCondition{Condition:%+v}", n.Condition)
}

type EqCondition struct {
	Field string `json:"field"`
	Value any    `json:"eq"`
}

func (e *EqCondition) Validate() error {
	if e.Field == "" {
		return errors.New("eq condition requires a field")
	}
	if str, ok := e.Value.(string); ok {
		ltCount := strings.Count(str, "<")
		gtCount := strings.Count(str, ">")

		if ltCount > 0 || gtCount > 0 {
			if ltCount != 1 || gtCount != 1 {
				return errors.New("eq value can contain only one placeholder embraced in '<' and '>'")
			}
			if strings.Index(str, "<") > strings.Index(str, ">") {
				return errors.New("'<' must come before '>'")
			}
		}
	}

	return nil
}

func (e *EqCondition) ToRedisQuery() (string, error) {
	switch v := e.Value.(type) {
	case string:
		return fmt.Sprintf(`@%s:{%s}`, e.Field, escapeString(v)), nil
	case float64, int:
		return fmt.Sprintf(`@%s:[%v %v]`, e.Field, v, v), nil
	default:
		return "", fmt.Errorf("unsupported value type for eq: %T", v)
	}
}

func (e *EqCondition) ReplacePlaceholder(vars map[string]any) (Condition, error) {
	if str, ok := e.Value.(string); ok {
		val, err := replacePlaceholderOrError(str, vars)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", e.Field, err)
		}
		return &EqCondition{
			Field: e.Field,
			Value: val,
		}, nil
	}
	return e, nil
}

func (e *EqCondition) String() string {
	return fmt.Sprintf("EqCondition{Field:%+v Value:%+v}", e.Field, e.Value)
}

type GtCondition struct {
	Field string `json:"field"`
	Value any    `json:"gt"`
}

func (g *GtCondition) Validate() error {
	if g.Field == "" {
		return errors.New("gt condition requires a field")
	}
	if str, ok := g.Value.(string); ok {
		ltCount := strings.Count(str, "<")
		gtCount := strings.Count(str, ">")

		if ltCount > 0 || gtCount > 0 {
			if ltCount != 1 || gtCount != 1 {
				return errors.New("gt value can contain only one placeholder embraced in '<' and '>'")
			}
			if strings.Index(str, "<") > strings.Index(str, ">") {
				return errors.New("'<' must come before '>'")
			}
		}
	}

	return nil
}

func (g *GtCondition) ToRedisQuery() (string, error) {
	switch v := g.Value.(type) {
	case float64:
		return fmt.Sprintf(`@%s:[(%f +inf]`, g.Field, v), nil
	case int:
		return fmt.Sprintf(`@%s:[(%d +inf]`, g.Field, v), nil
	default:
		return "", fmt.Errorf("unsupported value type for gt: %T", v)
	}
}
func (g *GtCondition) ReplacePlaceholder(vars map[string]any) (Condition, error) {
	if str, ok := g.Value.(string); ok {
		val, err := replacePlaceholderOrError(str, vars)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", g.Field, err)
		}
		return &GtCondition{
			Field: g.Field,
			Value: val,
		}, nil
	}
	return g, nil
}

func (g *GtCondition) String() string {
	return fmt.Sprintf("GtCondition{Field:%+v Value:%+v}", g.Field, g.Value)
}

type rawCondition struct {
	And *[]json.RawMessage `json:"AND"`
	Or  *[]json.RawMessage `json:"OR"`
	Not *json.RawMessage   `json:"NOT"`

	Field *string          `json:"field"`
	Eq    *json.RawMessage `json:"eq"`
	Gt    *json.RawMessage `json:"gt"`
}

func UnmarshalCondition(data []byte) (Condition, error) {
	var rc rawCondition
	if err := json.Unmarshal(data, &rc); err != nil {
		return nil, err
	}

	switch {
	case rc.And != nil:
		conds := make([]Condition, len(*rc.And))
		for i, raw := range *rc.And {
			c, err := UnmarshalCondition(raw)
			if err != nil {
				return nil, err
			}
			conds[i] = c
		}
		return &AndCondition{Conditions: conds}, nil

	case rc.Or != nil:
		conds := make([]Condition, len(*rc.Or))
		for i, raw := range *rc.Or {
			c, err := UnmarshalCondition(raw)
			if err != nil {
				return nil, err
			}
			conds[i] = c
		}
		return &OrCondition{Conditions: conds}, nil

	case rc.Not != nil:
		c, err := UnmarshalCondition(*rc.Not)
		if err != nil {
			return nil, err
		}
		return &NotCondition{Condition: c}, nil

	case rc.Eq != nil:
		if rc.Field == nil {
			return nil, errors.New("eq operator requires field")
		}
		var val any
		if err := json.Unmarshal(*rc.Eq, &val); err != nil {
			return nil, err
		}
		return &EqCondition{
			Field: *rc.Field,
			Value: val,
		}, nil

	case rc.Gt != nil:
		if rc.Field == nil {
			return nil, errors.New("gt operator requires field")
		}
		var val any
		if err := json.Unmarshal(*rc.Gt, &val); err != nil {
			return nil, err
		}
		return &GtCondition{
			Field: *rc.Field,
			Value: val,
		}, nil

	default:
		return nil, errors.New("unknown condition type or empty condition")
	}
}

type ConditionWrapper struct {
	Condition Condition
}

func (cw *ConditionWrapper) Validate() error {
	if cw == nil || cw.Condition == nil {
		return errors.New("cannot validate an empty condition")
	}
	return cw.Condition.Validate()
}

func (cw *ConditionWrapper) UnmarshalJSON(data []byte) error {
	cond, err := UnmarshalCondition(data)
	if err != nil {
		return err
	}
	cw.Condition = cond
	return nil
}

func (cw *ConditionWrapper) MarshalJSON() ([]byte, error) {
	if cw.Condition == nil {
		return []byte("null"), nil
	}

	switch cond := cw.Condition.(type) {
	case *AndCondition, *OrCondition, *NotCondition, *EqCondition, *GtCondition:
		return utils.MarshalJSONUnescaped(cond)
	default:
		return nil, errors.New("unsupported condition type")
	}
}

func (cw *ConditionWrapper) String() string {
	return fmt.Sprintf("%+v", cw.Condition)
}

func replacePlaceholderOrError(s string, vars map[string]any) (any, error) {
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		key := s[1 : len(s)-1]
		if val, ok := vars[key]; ok {
			return val, nil
		}
		return "", fmt.Errorf("unresolved placeholder: <%s>", key)
	}
	return s, nil
}

func escapeString(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}
