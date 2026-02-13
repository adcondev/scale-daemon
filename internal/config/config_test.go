package config

import (
	"reflect"
	"testing"
)

func TestGetEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		expected Environment
	}{
		{
			name:     "local environment",
			env:      "local",
			expected: Environments["local"],
		},
		{
			name:     "remote environment",
			env:      "remote",
			expected: Environments["remote"],
		},
		{
			name:     "unknown environment defaults to remote",
			env:      "unknown",
			expected: Environments["remote"],
		},
		{
			name:     "empty environment defaults to remote",
			env:      "",
			expected: Environments["remote"],
		},
		{
			name:     "prod environment (not in map) defaults to remote",
			env:      "prod",
			expected: Environments["remote"],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEnvironment(tt.env)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("GetEnvironment(%q) = %v, want %v", tt.env, got, tt.expected)
			}
		})
	}
}
