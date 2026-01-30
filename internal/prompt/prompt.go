// Package prompt provides interactive prompt functions for user input.
//
// This package provides utilities for prompting users for input via stdin,
// with support for email, password, and password confirmation prompts.
package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Email prompts the user for an email address with basic validation.
//
// Parameters:
//   - prompt: The prompt text to display
//
// Returns the user's input or an error if reading fails.
// Will reprompt if user provides invalid email format.
func Email(prompt string) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s: ", prompt)

		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)

		// Basic email validation
		if !strings.Contains(input, "@") || !strings.Contains(input, ".") {
			fmt.Println("Please enter a valid email address.")
			continue
		}

		return input, nil
	}
}

// Password prompts the user for a password (input is hidden).
//
// Parameters:
//   - prompt: The prompt text to display
//
// Returns the user's input or an error if reading fails.
// Uses terminal raw mode to hide password input.
func Password(prompt string) (string, error) {
	fmt.Printf("%s: ", prompt)

	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	fmt.Println() // Add newline after password input
	return string(password), nil
}

// ConfirmPassword prompts the user to confirm a password.
//
// The function will reprompt up to 3 times if passwords don't match.
//
// Returns an error if passwords don't match after max attempts.
func ConfirmPassword(prompt string, password string) error {
	const maxAttempts = 3

	for i := 0; i < maxAttempts; i++ {
		confirm, err := Password(prompt)
		if err != nil {
			return err
		}

		if confirm == password {
			return nil
		}

		if i < maxAttempts-1 {
			fmt.Println("Passwords do not match. Please try again.")
		}
	}

	return fmt.Errorf("password confirmation failed after %d attempts", maxAttempts)
}
