package utils

import "strings"

const (
	defaultRegistry   = "docker.io"
	defaultNamespace  = "library"
	defaultTag        = "latest"
	digestSeparator   = "@"
	tagSeparator      = ":"
	pathSeparator     = "/"
	registryDotChar   = "."
	registryPortChar  = ":"
	localhostRegistry = "localhost"
)

// NormalizeImageName normalizes image name to match Kubernetes behavior.
// Examples:
//   - "busybox" -> "docker.io/library/busybox:latest"
//   - "busybox:1.36.0" -> "docker.io/library/busybox:1.36.0"
//   - "myuser/myimage" -> "docker.io/myuser/myimage:latest"
//   - "gcr.io/project/image" -> "gcr.io/project/image:latest"
//   - "image@sha256:abc" -> "docker.io/library/image@sha256:abc"
func NormalizeImageName(imageName string) (normalized string) {
	if imageName == "" {
		return
	}

	var (
		registry  string
		path      string
		reference string
	)

	// Extract digest or tag
	var nameWithoutRef string
	if idx := strings.Index(imageName, digestSeparator); idx != -1 {
		nameWithoutRef = imageName[:idx]
		reference = imageName[idx:] // includes '@'
	} else if idx := strings.LastIndex(imageName, tagSeparator); idx != -1 {
		// Use LastIndex to handle registry with port like "localhost:5000/image:tag"
		parts := strings.SplitN(imageName, pathSeparator, 2)
		if len(parts) == 2 && isRegistry(parts[0]) {
			// Has registry, find tag after first '/'
			if tagIdx := strings.LastIndex(parts[1], tagSeparator); tagIdx != -1 {
				nameWithoutRef = imageName[:len(parts[0])+1+tagIdx]
				reference = imageName[len(parts[0])+1+tagIdx:]
			} else {
				nameWithoutRef = imageName
			}
		} else {
			// No registry, safe to use last ':'
			nameWithoutRef = imageName[:idx]
			reference = imageName[idx:]
		}
	} else {
		nameWithoutRef = imageName
	}

	// Split registry and path
	parts := strings.SplitN(nameWithoutRef, pathSeparator, 2)
	if len(parts) == 1 {
		// No '/', no registry
		// e.g., "busybox"
		registry = defaultRegistry
		path = parts[0]
	} else if isRegistry(parts[0]) {
		// First part is registry
		// e.g., "docker.io/library/busybox" or "localhost:5000/image"
		registry = parts[0]
		path = parts[1]
	} else {
		// First part is namespace under default registry
		// e.g., "myuser/myimage"
		registry = defaultRegistry
		path = nameWithoutRef
	}

	// Add default namespace for docker.io
	if registry == defaultRegistry && !strings.Contains(path, pathSeparator) {
		path = defaultNamespace + pathSeparator + path
	}

	// Add default tag if no reference
	if reference == "" {
		reference = tagSeparator + defaultTag
	}

	normalized = registry + pathSeparator + path + reference
	return
}

// isRegistry checks if the string looks like a registry domain
func isRegistry(part string) (result bool) {
	result = strings.Contains(part, registryDotChar) ||
		strings.Contains(part, registryPortChar) ||
		part == localhostRegistry
	return
}
