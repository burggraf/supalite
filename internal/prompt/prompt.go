// Package prompt provides interactive prompt functions for user input.
//
// This package provides utilities for prompting users for input via stdin,
// with support for string, password, and confirmation prompts.
package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Reader is the interface for reading user input.
type Reader struct {
	reader *bufio.Reader
}

// NewReader creates a new prompt reader.
func NewReader() *Reader {
	return &Reader{
		reader: bufio.NewReader(os.Stdin),
	}
}

// String prompts the user for a string value.
//
// Parameters:
//   - label: The prompt label/text
//   - current: The current value (if any)
//   - defaultVal: The default value if user presses enter
//
// Returns the user's input or the default value.
func (r *Reader) String(label, current, defaultVal string) string {
	if current != "" {
		defaultVal = current
	}

	prompt := fmt.Sprintf("%s", label)
	if defaultVal != "" {
		prompt += fmt.Sprintf(" [%s]", defaultVal)
	}
	prompt += ": "

	fmt.Print(prompt)
	input, _ := r.reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

// Password prompts the user for a password (input is hidden).
//
// Parameters:
//   - label: The prompt label/text
//   - current: The current value (if any)
//
// Returns the user's input or empty string if user presses enter.
// Note: On most terminals, this will NOT hide input - for better security
// consider using a terminal-aware library like gopass.
func (r *Reader) Password(label, current string) string {
	defaultVal := current

	prompt := fmt.Sprintf("%s", label)
	if defaultVal != "" {
		prompt += fmt.Sprintf(" [****]")
	}
	prompt += ": "

	fmt.Print(prompt)
	input, _ := r.reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

// ConfirmPassword prompts the user to confirm a password.
//
// Parameters:
//   - label: The prompt label/text
//   - password: The password to confirm
//
// Returns the confirmed password or an error if they don't match.
// The function will reprompt up to 3 times if passwords don't match.
func (r *Reader) ConfirmPassword(label, password string) error {
	const maxAttempts = 3

	for i := 0; i < maxAttempts; i++ {
		fmt.Printf("%s: ", label)
		input, _ := r.reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == password {
			return nil
		}

		if i < maxAttempts-1 {
			fmt.Println("Passwords do not match. Please try again.")
		}
	}

	return fmt.Errorf("password confirmation failed after %d attempts", maxAttempts)
}

// Bool prompts the user for a yes/no value.
//
// Parameters:
//   - label: The prompt label/text
//   - current: The current value (if any)
//   - defaultVal: The default value if user presses enter
//
// Returns true if user enters y/yes, false if n/no or default.
func (r *Reader) Bool(label string, current, defaultVal bool) bool {
	defaultValStr := "n"
	if defaultVal {
		defaultValStr = "y"
	}

	prompt := fmt.Sprintf("%s", label)
	prompt += fmt.Sprintf(" [%s]", defaultValStr)
	prompt += ": "

	fmt.Print(prompt)
	input, _ := r.reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" {
		return defaultVal
	}
	return input == "y" || input == "yes"
}

// Email prompts the user for an email address with basic validation.
//
// Parameters:
//   - label: The prompt label/text
//   - current: The current value (if any)
//   - defaultVal: The default value if user presses enter
//   - required: Whether the email is required (non-empty)
//
// Returns the user's input or the default value.
// Will reprompt if required and user provides empty input.
func (r *Reader) Email(label, current, defaultVal string, required bool) string {
	for {
		value := r.String(label, current, defaultVal)

		if value == "" && !required {
			return value
		}

		if value == "" && required {
			fmt.Println("Email is required. Please enter an email address.")
			continue
		}

		// Basic email validation
		if !strings.Contains(value, "@") || !strings.Contains(value, ".") {
			fmt.Println("Please enter a valid email address.")
			continue
		}

		return value
	}
}
