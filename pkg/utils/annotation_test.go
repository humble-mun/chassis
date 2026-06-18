package utils

import (
	"testing"
)

func TestValidateAnnotationKeyPrefix(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		wantErr bool
	}{
		{
			name:    "valid simple subdomain",
			prefix:  "example.com",
			wantErr: false,
		},
		{
			name:    "valid multi-label subdomain",
			prefix:  "pv-snapshotter.humble-mun.io",
			wantErr: false,
		},
		{
			name:    "valid single label",
			prefix:  "example",
			wantErr: false,
		},
		{
			name:    "trailing slash rejected",
			prefix:  "example.com/",
			wantErr: true,
		},
		{
			name:    "uppercase rejected",
			prefix:  "Example.com",
			wantErr: true,
		},
		{
			name:    "contains slash rejected",
			prefix:  "example.com/foo",
			wantErr: true,
		},
		{
			name:    "empty rejected",
			prefix:  "",
			wantErr: true,
		},
		{
			name:    "leading dot rejected",
			prefix:  ".example.com",
			wantErr: true,
		},
		{
			name:    "reserved kubernetes.io rejected",
			prefix:  "kubernetes.io",
			wantErr: true,
		},
		{
			name:    "reserved k8s.io rejected",
			prefix:  "k8s.io",
			wantErr: true,
		},
		{
			name:    "subdomain of kubernetes.io rejected",
			prefix:  "node.kubernetes.io",
			wantErr: true,
		},
		{
			name:    "subdomain of k8s.io rejected",
			prefix:  "foo.k8s.io",
			wantErr: true,
		},
		{
			name:    "lookalike of reserved domain allowed",
			prefix:  "myk8s.io",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAnnotationKeyPrefix(tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAnnotationKeyPrefix(%q) error = %v, wantErr %v",
					tt.prefix, err, tt.wantErr)
			}
		})
	}
}
