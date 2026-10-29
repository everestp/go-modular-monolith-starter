// Package validator contains small, reusable validation helpers.
package validator

import "strings"

// MinLength ensures a string exceeds a minimum length.
func MinLength(v string, minimum int) bool { return len(strings.TrimSpace(v)) >= minimum }

// IsEmail checks a basic email format.
func IsEmail(v string) bool { return strings.Contains(v, "@") && strings.Contains(v, ".") }
