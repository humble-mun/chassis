package utils

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	// reservedAnnotationPrefixKubernetes and reservedAnnotationPrefixK8s are the
	// two Kubernetes-reserved DNS subdomains that must not be used as a custom
	// annotation key prefix, to avoid conflicts with built-in annotations.
	reservedAnnotationPrefixKubernetes = "kubernetes.io"
	reservedAnnotationPrefixK8s        = "k8s.io"
)

// ValidateAnnotationKeyPrefix checks that prefix is a valid Kubernetes
// annotation key prefix (RFC 1123 DNS subdomain, ≤253 chars, no trailing
// slash).
//
// A Kubernetes annotation key has the form "<prefix>/<name>"; the prefix
// segment, when present, must be a DNS subdomain. Components that let users
// supply a custom annotation key prefix should validate it with this function
// before composing annotation keys.
//
// The reserved prefixes "kubernetes.io" and "k8s.io" (and any subdomain of
// them) are rejected to avoid conflicts with built-in Kubernetes annotations.
func ValidateAnnotationKeyPrefix(prefix string) error {
	if strings.HasSuffix(prefix, "/") {
		return fmt.Errorf("annotation prefix must not include a trailing slash: %q", prefix)
	}
	if errs := validation.IsDNS1123Subdomain(prefix); len(errs) > 0 {
		return fmt.Errorf("annotation prefix %q is not a valid DNS subdomain: %s",
			prefix, strings.Join(errs, "; "))
	}
	if prefix == reservedAnnotationPrefixKubernetes || prefix == reservedAnnotationPrefixK8s ||
		strings.HasSuffix(prefix, "."+reservedAnnotationPrefixKubernetes) || strings.HasSuffix(prefix, "."+reservedAnnotationPrefixK8s) {
		return fmt.Errorf("annotation prefix %q uses a reserved Kubernetes domain", prefix)
	}
	return nil
}
