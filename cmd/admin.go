package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/markb/supalite/internal/admin"
	"github.com/markb/supalite/internal/config"
	"github.com/markb/supalite/internal/pg"
	"github.com/markb/supalite/internal/prompt"
	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Manage admin users",
	Long:  `Manage admin users for the Supalite admin dashboard.`,
}

var adminAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new admin user",
	Long: `Add a new admin user to the database.

You will be prompted for an email address and password.`,
	RunE: runAdminAdd,
}

var adminChangePasswordCmd = &cobra.Command{
	Use:   "change-password",
	Short: "Change an admin user's password",
	Long: `Change an admin user's password.

You will be prompted for the email address and new password.`,
	RunE: runAdminChangePassword,
}

var adminDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an admin user",
	Long: `Delete an admin user from the database.

You will be prompted for the email address and confirmation.`,
	RunE: runAdminDelete,
}

var adminListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all admin users",
	Long:  `List all admin users in the database.`,
	RunE: runAdminList,
}

func init() {
	rootCmd.AddCommand(adminCmd)
	adminCmd.AddCommand(adminAddCmd)
	adminCmd.AddCommand(adminChangePasswordCmd)
	adminCmd.AddCommand(adminDeleteCmd)
	adminCmd.AddCommand(adminListCmd)
}

// runAdminAdd adds a new admin user
func runAdminAdd(cmd *cobra.Command, args []string) error {
	fmt.Println("===========================================")
	fmt.Println("Add Admin User")
	fmt.Println("===========================================")
	fmt.Println()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Prompt for email
	email, err := prompt.Email("Email")
	if err != nil {
		return fmt.Errorf("failed to read email: %w", err)
	}

	// Prompt for password
	password, err := prompt.Password("Password")
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	// Confirm password
	if err := prompt.ConfirmPassword("Confirm password", password); err != nil {
		return err
	}

	fmt.Println()

	// Connect to database
	conn, ctx, cleanup, err := connectToDatabase(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	// Create user
	user, err := admin.Create(ctx, conn, email, password)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	fmt.Printf("✓ Admin user created successfully!\n")
	fmt.Printf("  Email: %s\n", user.Email)
	fmt.Printf("  ID: %s\n", user.ID)
	fmt.Printf("  Created: %s\n", user.CreatedAt.Format(time.RFC3339))

	return nil
}

// runAdminChangePassword changes an admin user's password
func runAdminChangePassword(cmd *cobra.Command, args []string) error {
	fmt.Println("===========================================")
	fmt.Println("Change Admin Password")
	fmt.Println("===========================================")
	fmt.Println()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Prompt for email
	email, err := prompt.Email("Email")
	if err != nil {
		return fmt.Errorf("failed to read email: %w", err)
	}

	// Prompt for new password
	newPassword, err := prompt.Password("New password")
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	if newPassword == "" {
		return fmt.Errorf("password cannot be empty")
	}

	// Confirm password
	if err := prompt.ConfirmPassword("Confirm new password", newPassword); err != nil {
		return err
	}

	fmt.Println()

	// Connect to database
	conn, ctx, cleanup, err := connectToDatabase(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	// Update password
	if err := admin.UpdatePassword(ctx, conn, email, newPassword); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	fmt.Printf("✓ Password updated successfully for: %s\n", email)

	return nil
}

// runAdminDelete deletes an admin user
func runAdminDelete(cmd *cobra.Command, args []string) error {
	fmt.Println("===========================================")
	fmt.Println("Delete Admin User")
	fmt.Println("===========================================")
	fmt.Println()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Prompt for email
	email, err := prompt.Email("Email")
	if err != nil {
		return fmt.Errorf("failed to read email: %w", err)
	}

	fmt.Println()

	// Confirm deletion
	fmt.Print("Are you sure you want to delete this user? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
		fmt.Println("Deletion cancelled.")
		return nil
	}

	fmt.Println()

	// Connect to database
	conn, ctx, cleanup, err := connectToDatabase(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	// Delete user
	if err := admin.Delete(ctx, conn, email); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	fmt.Printf("✓ Admin user deleted: %s\n", email)

	return nil
}

// runAdminList lists all admin users
func runAdminList(cmd *cobra.Command, args []string) error {
	fmt.Println("===========================================")
	fmt.Println("Admin Users")
	fmt.Println("===========================================")
	fmt.Println()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Connect to database
	conn, ctx, cleanup, err := connectToDatabase(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	// List users
	users, err := admin.List(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	if len(users) == 0 {
		fmt.Println("No admin users found.")
		fmt.Println()
		fmt.Println("Create an admin user with:")
		fmt.Println("  ./supalite admin add")
		return nil
	}

	fmt.Printf("Found %d admin user(s):\n", len(users))
	fmt.Println()
	for i, user := range users {
		fmt.Printf("%d. %s\n", i+1, user.Email)
		fmt.Printf("   ID: %s\n", user.ID)
		fmt.Printf("   Created: %s\n", user.CreatedAt.Format(time.RFC3339))
		fmt.Printf("   Updated: %s\n", user.UpdatedAt.Format(time.RFC3339))
		fmt.Println()
	}

	return nil
}

// connectToDatabase establishes a connection to the embedded database
func connectToDatabase(cfg *config.Config) (*pgx.Conn, context.Context, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	// Create embedded database config
	dbCfg := pg.Config{
		Port:     cfg.PGPort,
		Username: cfg.PGUsername,
		Password: cfg.PGPassword,
		Database: cfg.PGDatabase,
		DataDir:  cfg.DataDir,
	}

	db := pg.NewEmbeddedDatabase(dbCfg)

	// Start database
	fmt.Printf("Starting database...\n")
	if err := db.Start(ctx); err != nil {
		cancel()
		return nil, nil, nil, fmt.Errorf("failed to start database: %w", err)
	}

	// Connect to database
	conn, err := db.Connect(ctx)
	if err != nil {
		cancel()
		db.Stop()
		return nil, nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Cleanup function
	cleanup := func() {
		conn.Close(ctx)
		db.Stop()
		cancel()
	}

	return conn, ctx, cleanup, nil
}
