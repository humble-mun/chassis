package utils

import (
	"testing"
)

func TestNormalizeImageName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "image name only",
			input: "busybox",
			want:  "docker.io/library/busybox:latest",
		},
		{
			name:  "image name with tag",
			input: "busybox:1.36.0",
			want:  "docker.io/library/busybox:1.36.0",
		},
		{
			name:  "dockerhub user/image",
			input: "myuser/myimage",
			want:  "docker.io/myuser/myimage:latest",
		},
		{
			name:  "dockerhub user/image with tag",
			input: "myuser/myimage:v1.0",
			want:  "docker.io/myuser/myimage:v1.0",
		},
		{
			name:  "explicit docker.io",
			input: "docker.io/library/busybox:1.36.0",
			want:  "docker.io/library/busybox:1.36.0",
		},
		{
			name:  "gcr.io image",
			input: "gcr.io/my-project/image",
			want:  "gcr.io/my-project/image:latest",
		},
		{
			name:  "gcr.io image with tag",
			input: "gcr.io/my-project/image:v1.0",
			want:  "gcr.io/my-project/image:v1.0",
		},
		{
			name:  "localhost registry",
			input: "localhost/image:latest",
			want:  "localhost/image:latest",
		},
		{
			name:  "localhost with port",
			input: "localhost:5000/image:latest",
			want:  "localhost:5000/image:latest",
		},
		{
			name:  "registry with port and multi-path",
			input: "registry.example.com:5000/project/image:v1",
			want:  "registry.example.com:5000/project/image:v1",
		},
		{
			name:  "image with digest",
			input: "busybox@sha256:abcd1234",
			want:  "docker.io/library/busybox@sha256:abcd1234",
		},
		{
			name:  "full name with digest",
			input: "gcr.io/project/image@sha256:abcd1234",
			want:  "gcr.io/project/image@sha256:abcd1234",
		},
		{
			name:  "registry.k8s.io",
			input: "registry.k8s.io/kube-apiserver:v1.28.0",
			want:  "registry.k8s.io/kube-apiserver:v1.28.0",
		},
		{
			name:  "multi-level path",
			input: "gcr.io/project/subproject/image:latest",
			want:  "gcr.io/project/subproject/image:latest",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeImageName(tt.input); got != tt.want {
				t.Errorf("NormalizeImageName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
