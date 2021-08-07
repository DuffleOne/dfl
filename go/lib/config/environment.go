package config

// Getter takes an environment variable name, and returns its
// value or "" if not set.
type Getter func(string) string

// EnvironmentName returns the name of the current execution environment
// from CONFIG. If no environment is detected, "local" is returned.
func EnvironmentName(get Getter) (env string) {
	env = get("ENV")

	if env == "" {
		return "local"
	}

	return
}
