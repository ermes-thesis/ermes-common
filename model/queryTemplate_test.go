package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomPrint(t *testing.T) {
	templ := QueryTemplate{
		Select: "ALL",
		As:     "test",
		Where: &ConditionWrapper{
			Condition: &AndCondition{
				Conditions: []Condition{
					&EqCondition{
						Field: "type",
						Value: "sensor",
					},
					&NotCondition{
						Condition: &GtCondition{
							Field: "test",
							Value: "test_value",
						},
					},
					&OrCondition{
						Conditions: []Condition{
							&GtCondition{
								Field: "pippo",
								Value: "pluto",
							},
							&EqCondition{
								Field: "paperino",
								Value: "qui",
							},
						},
					},
				},
			},
		},
	}
	fmt.Printf("%+v", templ)
}

func TestQueryTemplateValidate(t *testing.T) {
	tests := []struct {
		name      string
		template  QueryTemplate
		wantedErr string
	}{
		{
			name:      "missing SELECT",
			template:  QueryTemplate{As: "test"},
			wantedErr: "SELECT field is required",
		},
		{
			name:      "invalid SELECT",
			template:  QueryTemplate{Select: "invalid"},
			wantedErr: "invalid select value: invalid",
		},
		{
			name:      "missing AS",
			template:  QueryTemplate{Select: "ALL"},
			wantedErr: "AS field is required",
		},
		{
			name:      "valid template without WHERE",
			template:  QueryTemplate{Select: "ALL", As: "test"},
			wantedErr: "",
		},
		{
			name: "valid template with WHERE",
			template: QueryTemplate{
				Select: "ALL",
				As:     "test",
				Where: &ConditionWrapper{
					Condition: &EqCondition{
						Field: "type",
						Value: "sensor",
					},
				},
			},
			wantedErr: "",
		},
		{
			name: "invalid template with wrong condition in Eq Condition",
			template: QueryTemplate{
				Select: "ALL",
				As:     "test",
				Where: &ConditionWrapper{
					Condition: &EqCondition{
						Field: "",
						Value: "sensor",
					},
				},
			},
			wantedErr: "eq condition requires a field",
		},
		{
			name: "invalid template placeholder in Eq Condition",
			template: QueryTemplate{
				Select: "ALL",
				As:     "test",
				Where: &ConditionWrapper{
					Condition: &EqCondition{
						Field: "test",
						Value: "<sensor><sensor2>",
					},
				},
			},
			wantedErr: "eq value can contain only one placeholder embraced in '<' and '>'",
		},
		{
			name: "wrong template placeholder delimiters in Eq Condition",
			template: QueryTemplate{
				Select: "ALL",
				As:     "test",
				Where: &ConditionWrapper{
					Condition: &EqCondition{
						Field: "test",
						Value: ">sensor<",
					},
				},
			},
			wantedErr: "'<' must come before '>'",
		},
		{
			name: "invalid template with wrong condition in Gt Condition",
			template: QueryTemplate{
				Select: "ALL",
				As:     "test",
				Where: &ConditionWrapper{
					Condition: &GtCondition{
						Field: "",
						Value: "sensor",
					},
				},
			},
			wantedErr: "gt condition requires a field",
		},
		{
			name: "invalid template placeholder in Gt Condition",
			template: QueryTemplate{
				Select: "ALL",
				As:     "test",
				Where: &ConditionWrapper{
					Condition: &GtCondition{
						Field: "test",
						Value: "<sensor",
					},
				},
			},
			wantedErr: "gt value can contain only one placeholder embraced in '<' and '>'",
		},
		{
			name: "wrong template placeholder delimiters in Gt Condition",
			template: QueryTemplate{
				Select: "ALL",
				As:     "test",
				Where: &ConditionWrapper{
					Condition: &GtCondition{
						Field: "test",
						Value: ">sensor<",
					},
				},
			},
			wantedErr: "'<' must come before '>'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.template.Validate()
			if tt.wantedErr != "" {
				require.Equal(t, tt.wantedErr, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnmarshalCondition(t *testing.T) {
	jsonStr := `{
			"AND": [
				{"field":"test","eq":"value"},
				{"field":"test2","eq":"value2"}
			]
	}`

	cond, err := UnmarshalCondition([]byte(jsonStr))
	require.NoError(t, err)

	and, ok := cond.(*AndCondition)
	require.True(t, ok)
	require.Len(t, and.Conditions, 2)

	eq1, ok := and.Conditions[0].(*EqCondition)
	require.True(t, ok)
	require.Equal(t, eq1.Field, "test")
	require.Equal(t, eq1.Value, "value")
	eq2, ok := and.Conditions[1].(*EqCondition)
	require.True(t, ok)
	require.Equal(t, eq2.Field, "test2")
	require.Equal(t, eq2.Value, "value2")
}

type UnsupportedCondition struct {
	Field string
	Value any
}

func (c UnsupportedCondition) ToRedisQuery() (string, error) {
	panic("implement me")
}

func (c UnsupportedCondition) ReplacePlaceholder(vars map[string]any) (Condition, error) {
	panic("implement me")
}

func (c UnsupportedCondition) Validate() error {
	if c.Field == "" {
		return errors.New("error")
	}
	return nil
}

func TestConditionWrapper_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name          string
		condition     ConditionWrapper
		wantedErr     string
		validationErr string
	}{
		{
			name: "AND Condition w/ Eq, Gt conditions",
			condition: ConditionWrapper{
				Condition: &AndCondition{
					Conditions: []Condition{
						&EqCondition{Field: "test", Value: "value"},
						&GtCondition{Field: "test2", Value: "value2"},
					},
				},
			},
			wantedErr: "",
		},
		{
			name: "OR Condition w/ Eq, Gt conditions",
			condition: ConditionWrapper{
				Condition: &OrCondition{
					Conditions: []Condition{
						&EqCondition{Field: "test", Value: "value"},
						&GtCondition{Field: "test2", Value: "value2"},
					},
				},
			},
			wantedErr: "",
		},
		{
			name: "NOT Condition w/ Eq conditions",
			condition: ConditionWrapper{
				Condition: &NotCondition{
					Condition: &EqCondition{Field: "test", Value: "value"},
				},
			},
			wantedErr: "",
		},
		{
			name: "Only Eq condition",
			condition: ConditionWrapper{
				Condition: &EqCondition{Field: "test", Value: "value"},
			},
			wantedErr: "",
		},
		{
			name: "Only Gt condition",
			condition: ConditionWrapper{
				Condition: &GtCondition{Field: "test", Value: "value2"},
			},
			wantedErr: "",
		},
		{
			name: "Unsupported condition",
			condition: ConditionWrapper{
				Condition: UnsupportedCondition{
					Field: "test",
					Value: "value",
				},
			},
			wantedErr: "unsupported condition type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(&tt.condition)
			if tt.wantedErr != "" {
				require.Error(t, err, tt.wantedErr)
			} else {
				require.NoError(t, err)

				// Unmarshal
				var q ConditionWrapper
				err = json.Unmarshal(data, &q)
				require.NoError(t, err)

				err = q.Validate()
				if tt.validationErr == "" {
					require.NoError(t, err)
				} else {
					require.EqualError(t, err, tt.validationErr)
				}
			}
		})
	}
}

func TestConditionWrapper_InvalidUnmarshal(t *testing.T) {
	_, err := UnmarshalCondition([]byte(`{}`))
	require.EqualError(t, err, "unknown condition type or empty condition")
}
