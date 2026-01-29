package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/markb/supalite/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure Supalite settings",
	Long:  `Interactively configure Supalite settings such as email/SMTP.`,
}

var emailCmd = &cobra.Command{
	Use:   "email",
	Short: "Configure email/SMTP settings for GoTrue",
	Long: `Interactively configure email/SMTP settings for the GoTrue auth server.

This will prompt you for SMTP configuration and save it to supalite.json.
You can also choose capture mode to store emails in the database for development.`,
	RunE: runEmailConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(emailCmd)
}

// runEmailConfig runs the interactive email configuration wizard
func runEmailConfig(cmd *cobra.Command, args []string) error {
	fmt.Println("===========================================")
	fmt.Println("Supalite Email Configuration Wizard")
	fmt.Println("===========================================")
	fmt.Println()
	fmt.Println("This wizard will help you configure email settings for sending emails")
	fmt.Println("(email confirmations, password resets, etc.).")
	fmt.Println()
	fmt.Println("Your configuration will be saved to supalite.json")
	fmt.Println()

	// Load existing config if it exists
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load existing config: %w", err)
	}

	// Ensure email config exists
	if cfg.Email == nil {
		cfg.Email = &config.EmailConfig{}
	}

	reader := bufio.NewReader(os.Stdin)

	// Step 1: Choose email mode
	fmt.Println("\n=== Email Mode ===")
	fmt.Println("Choose email mode:")
	fmt.Println("  1. SMTP - Send real emails via SMTP server")
	fmt.Println("  2. Capture - Store emails in database (for development)")

	modeChoice := promptString(reader, "Enter choice", "", "1")
	captureMode := strings.TrimSpace(modeChoice) == "2"

	cfg.Email.CaptureMode = captureMode

	if captureMode {
		// Capture mode configuration
		cfg.Email.CapturePort = promptInt(reader, "Mail capture port", cfg.Email.CapturePort, 1025)

		fmt.Println()
		fmt.Println("Capture mode enabled.")
		fmt.Println("Emails will be stored in the database instead of being sent.")
		fmt.Println("Query captured emails: GET /rest/v1/captured_emails")
		fmt.Println()

		// In capture mode, we don't need SMTP config
		// Clear any existing SMTP credentials to avoid confusion
		cfg.Email.SMTPHost = ""
		cfg.Email.SMTPPort = 0
		cfg.Email.SMTPUser = ""
		cfg.Email.SMTPPass = ""
		cfg.Email.SMTPAdminEmail = ""
		cfg.Email.MailerAutoconfirm = false
	} else {
		// SMTP mode configuration
		cfg.Email.CaptureMode = false
		cfg.Email.CapturePort = 0

		fmt.Println()
		fmt.Println("=== SMTP Configuration ===")

		// Prompt for each field
		cfg.Email.SMTPHost = promptString(reader, "SMTP host", cfg.Email.SMTPHost, "smtp.gmail.com")
		cfg.Email.SMTPPort = promptInt(reader, "SMTP port", cfg.Email.SMTPPort, 587)
		cfg.Email.SMTPUser = promptString(reader, "SMTP username", cfg.Email.SMTPUser, "")
		cfg.Email.SMTPPass = promptString(reader, "SMTP password", cfg.Email.SMTPPass, "")
		cfg.Email.SMTPAdminEmail = promptString(reader, "Admin email (for password resets)", cfg.Email.SMTPAdminEmail, "")
		cfg.Email.MailerAutoconfirm = promptBool(reader, "Skip email confirmation (autoconfirm)", cfg.Email.MailerAutoconfirm, false)
	}

	fmt.Println()

	// Sanity check the configuration
	warnings := validateEmailConfig(cfg.Email)
	if len(warnings) > 0 {
		fmt.Println("⚠️  Configuration Warnings:")
		for _, w := range warnings {
			fmt.Printf("  - %s\n", w)
		}
		fmt.Println()
	}

	// Ask for confirmation
	fmt.Println("Configuration Summary:")
	if cfg.Email.CaptureMode {
		fmt.Println("  Mode: Capture (development)")
		fmt.Printf("  Capture Port: %d\n", cfg.Email.CapturePort)
		fmt.Println()
		fmt.Println("  ⚠️  Emails will be stored in database, not sent!")
		fmt.Println("  Query captured emails: GET /rest/v1/captured_emails")
	} else {
		fmt.Println("  Mode: SMTP (production)")
		fmt.Printf("  SMTP Host: %s\n", cfg.Email.SMTPHost)
		fmt.Printf("  SMTP Port: %d\n", cfg.Email.SMTPPort)
		fmt.Printf("  SMTP User: %s\n", valueOrEmpty(cfg.Email.SMTPUser))
		fmt.Printf("  SMTP Pass: %s\n", valueOrEmpty(maskString(cfg.Email.SMTPPass)))
		fmt.Printf("  Admin Email: %s\n", valueOrEmpty(cfg.Email.SMTPAdminEmail))
		fmt.Printf("  Autoconfirm: %t\n", cfg.Email.MailerAutoconfirm)
	}
	fmt.Println()

	confirm := promptBool(reader, "Save this configuration to supalite.json?", true, true)
	if !confirm {
		fmt.Println("Configuration cancelled.")
		return nil
	}

	// Write configuration to file
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println()
	fmt.Println("✓ Email configuration saved to supalite.json")
	fmt.Println()
	fmt.Println("You can now start Supalite with:")
	fmt.Println("  ./supalite serve")
	fmt.Println()

	return nil
}

// promptString prompts the user for a string value
func promptString(reader *bufio.Reader, label string, current, defaultVal string) string {
	if current != "" {
		defaultVal = current
	}

	prompt := fmt.Sprintf("%s", label)
	if defaultVal != "" {
		prompt += fmt.Sprintf(" [%s]", defaultVal)
	}
	prompt += ": "

	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

// promptInt prompts the user for an integer value
func promptInt(reader *bufio.Reader, label string, current, defaultVal int) int {
	if current != 0 {
		defaultVal = current
	}

	prompt := fmt.Sprintf("%s", label)
	if defaultVal != 0 {
		prompt += fmt.Sprintf(" [%d]", defaultVal)
	}
	prompt += ": "

	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}

	var val int
	if _, err := fmt.Sscanf(input, "%d", &val); err == nil {
		return val
	}
	return defaultVal
}

// promptBool prompts the user for a yes/no value
func promptBool(reader *bufio.Reader, label string, current, defaultVal bool) bool {
	defaultValStr := "n"
	if defaultVal {
		defaultValStr = "y"
	}

	prompt := fmt.Sprintf("%s", label)
	prompt += fmt.Sprintf(" [%s]", defaultValStr)
	prompt += ": "

	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" {
		return defaultVal
	}
	return input == "y" || input == "yes"
}

// validateEmailConfig performs sanity checks on the email configuration
func validateEmailConfig(email *config.EmailConfig) []string {
	var warnings []string

	// In capture mode, we don't need SMTP configuration
	if email.CaptureMode {
		return warnings
	}

	// Check for missing required fields
	if email.SMTPHost == "" {
		warnings = append(warnings, "SMTP host is not set - emails will not be sent")
	}
	if email.SMTPPort == 0 {
		if email.SMTPHost != "" {
			warnings = append(warnings, "SMTP port is not set - using default port 587")
		}
	} else if email.SMTPPort != 25 && email.SMTPPort != 465 && email.SMTPPort != 587 {
		warnings = append(warnings, fmt.Sprintf("Unusual SMTP port %d (common ports: 25, 465, 587)", email.SMTPPort))
	}

	if email.SMTPHost != "" && email.SMTPUser == "" {
		warnings = append(warnings, "SMTP username is not set - most SMTP servers require authentication")
	}
	if email.SMTPHost != "" && email.SMTPPass == "" {
		warnings = append(warnings, "SMTP password is not set - most SMTP servers require authentication")
	}

	if email.SMTPHost != "" && email.SMTPAdminEmail == "" {
		warnings = append(warnings, "Admin email is not set - password reset emails may not work properly")
	}

	// Check for common provider-specific issues
	if strings.Contains(email.SMTPHost, "gmail.com") && !strings.Contains(email.SMTPUser, "@gmail.com") {
		warnings = append(warnings, "Gmail requires your full email address as the username")
	}
	if strings.Contains(email.SMTPHost, "gmail.com") && email.SMTPPass != "" && len(email.SMTPPass) < 12 {
		warnings = append(warnings, "Gmail requires an App Password (16 characters), not your regular password")
	}

	// Warn about autoconfirm mode
	if email.MailerAutoconfirm {
		warnings = append(warnings, "Autoconfirm is enabled - users will not receive confirmation emails (OK for development)")
	}

	return warnings
}

// saveConfig saves the configuration to supalite.json
func saveConfig(cfg *config.Config) error {
	// Marshal with indentation for readability
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile("supalite.json", data, 0644)
}

// valueOrEmpty returns the value or "(not set)" if empty
func valueOrEmpty(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

// maskString masks a string for display (e.g., passwords)
func maskString(s string) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}
