package validate

import "testing"

func TestValidateXApiGateway_xagwNotMap(t *testing.T) {
	t.Parallel()
	issues := ValidateXApiGateway(map[string]any{"x-api-gateway": 1})
	if len(issues) != 1 {
		t.Fatalf("got %#v", issues)
	}
}

func TestValidateXApiGateway_v1_fieldErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		xagw map[string]any
		minN int
	}{
		{
			name: "empty_prefix",
			xagw: map[string]any{
				"version": "v1", "prefix": "  ", "allowUndefinedMethods": true,
				"contract": map[string]any{"name": "c"},
				"service":  map[string]any{"name": "s"},
				"access":   map[string]any{"secure": true},
			},
			minN: 1,
		},
		{
			name: "bool_bad",
			xagw: map[string]any{
				"version": "v1", "prefix": "/a/", "allowUndefinedMethods": "nope",
				"contract": map[string]any{"name": "c"},
				"service":  map[string]any{"name": "s"},
				"access":   map[string]any{"secure": true},
			},
			minN: 1,
		},
		{
			name: "contract_missing",
			xagw: map[string]any{
				"version": "v1", "prefix": "/a/", "allowUndefinedMethods": true,
				"service": map[string]any{"name": "s"},
				"access":  map[string]any{"secure": true},
			},
			minN: 1,
		},
		{
			name: "contract_name_empty",
			xagw: map[string]any{
				"version": "v1", "prefix": "/a/", "allowUndefinedMethods": true,
				"contract": map[string]any{"name": "  "},
				"service":  map[string]any{"name": "s"},
				"access":   map[string]any{"secure": true},
			},
			minN: 1,
		},
		{
			name: "contract_desc_bad_type",
			xagw: map[string]any{
				"version": "v1", "prefix": "/a/", "allowUndefinedMethods": true,
				"contract": map[string]any{"name": "c", "description": 1},
				"service":  map[string]any{"name": "s"},
				"access":   map[string]any{"secure": true},
			},
			minN: 1,
		},
		{
			name: "service_not_map",
			xagw: map[string]any{
				"version": "v1", "prefix": "/a/", "allowUndefinedMethods": true,
				"contract": map[string]any{"name": "c"},
				"service":  1,
				"access":   map[string]any{"secure": true},
			},
			minN: 1,
		},
		{
			name: "access_apps_not_array",
			xagw: map[string]any{
				"version": "v1", "prefix": "/a/", "allowUndefinedMethods": true,
				"contract": map[string]any{"name": "c"},
				"service":  map[string]any{"name": "s"},
				"access":   map[string]any{"secure": true, "apps": "x"},
			},
			minN: 1,
		},
		{
			name: "access_app_item_not_map",
			xagw: map[string]any{
				"version": "v1", "prefix": "/a/", "allowUndefinedMethods": true,
				"contract": map[string]any{"name": "c"},
				"service":  map[string]any{"name": "s"},
				"access":   map[string]any{"secure": true, "apps": []any{1}},
			},
			minN: 1,
		},
		{
			name: "access_envs_not_array",
			xagw: map[string]any{
				"version": "v1", "prefix": "/a/", "allowUndefinedMethods": true,
				"contract": map[string]any{"name": "c"},
				"service":  map[string]any{"name": "s"},
				"access": map[string]any{
					"secure": true,
					"apps": []any{map[string]any{
						"app_id":       "a",
						"environments": 1,
					}},
				},
			},
			minN: 1,
		},
		{
			name: "access_env_elem_not_string",
			xagw: map[string]any{
				"version": "v1", "prefix": "/a/", "allowUndefinedMethods": true,
				"contract": map[string]any{"name": "c"},
				"service":  map[string]any{"name": "s"},
				"access": map[string]any{
					"secure": true,
					"apps": []any{map[string]any{
						"app_id":       "a",
						"environments": []any{1},
					}},
				},
			},
			minN: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := map[string]any{"x-api-gateway": tc.xagw}
			issues := ValidateXApiGateway(root)
			if len(issues) < tc.minN {
				t.Fatalf("expected at least %d issues, got %#v", tc.minN, issues)
			}
		})
	}
}

func TestAsBool(t *testing.T) {
	t.Parallel()
	if b, ok := asBool(true); !ok || !b {
		t.Fatal()
	}
	if _, ok := asBool(1); ok {
		t.Fatal()
	}
}
