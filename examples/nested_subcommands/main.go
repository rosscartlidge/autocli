package main

import (
	"fmt"
	"os"

	cf "github.com/rosscartlidge/autocli/v4"
)

func main() {
	cmd := cf.NewCommand("gitlike").
		Version("1.0.0").
		Description("A git-like CLI demonstrating nested subcommands").
		Author("Example Author").
		Flag("-verbose", "-v").Bool().Global().Help("Enable verbose output").Done().
		Flag("-config").String().Global().Help("Path to config file").Done().

		// Top-level subcommand: remote
		Subcommand("remote").
			Description("Manage remote repositories").

			// Nested subcommand: remote add
			Subcommand("add").
				Description("Add a new remote repository").
				Flag("-fetch", "-f").Bool().Help("Fetch after adding").Done().
				Flag("-tags", "-t").Bool().Help("Import tags").Done().
				Handler(handleCommand).
				Done().

			// Nested subcommand: remote remove
			Subcommand("remove").
				Description("Remove a remote repository").
				Flag("-force").Bool().Help("Force removal even if remote is in use").Done().
				Handler(handleCommand).
				Done().

			// Nested subcommand: remote list
			Subcommand("list").
				Description("List all remote repositories (use global -verbose for URLs)").
				Handler(handleCommand).
				Done().

			Done().

		// Top-level subcommand: branch
		Subcommand("branch").
			Description("Manage branches").

			// Nested subcommand: branch list
			Subcommand("list").
				Description("List all branches").
				Flag("-all", "-a").Bool().Help("List all branches including remotes").Done().
				Handler(handleCommand).
				Done().

			// Nested subcommand: branch delete
			Subcommand("delete").
				Description("Delete a branch").
				Flag("-force", "-f").Bool().Help("Force deletion").Done().
				Handler(handleCommand).
				Done().

			Done().

		// Top-level subcommand: config
		Subcommand("config").
			Description("Get and set configuration options").

			// Nested subcommand: config get
			Subcommand("get").
				Description("Get a configuration value").
				Handler(handleCommand).
				Done().

			// Nested subcommand: config set
			Subcommand("set").
				Description("Set a configuration value").
				Flag("-global").Bool().Help("Set global configuration").Done().
				Handler(handleCommand).
				Done().

			// Nested subcommand: config list
			Subcommand("list").
				Description("List all configuration values").
				Handler(handleCommand).
				Done().

			Done().

		Example("gitlike remote add origin https://github.com/user/repo.git", "Add a new remote named origin").
		Example("gitlike -verbose remote list", "List all remotes with verbose output").
		Example("gitlike branch delete -force old-feature", "Force delete a branch").
		Example("gitlike config set -global user.name \"John Doe\"", "Set global username").
		Build()

	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleCommand(ctx *cf.Context) error {
	verbose := ctx.GetBool("-verbose", false)
	config := ctx.GetString("-config", "")

	if verbose {
		fmt.Printf("Verbose mode enabled\n")
	}
	if config != "" {
		fmt.Printf("Using config file: %s\n", config)
	}

	// Handle different subcommand paths
	switch {
	case ctx.IsSubcommandPath("remote", "add"):
		return handleRemoteAdd(ctx)
	case ctx.IsSubcommandPath("remote", "remove"):
		return handleRemoteRemove(ctx)
	case ctx.IsSubcommandPath("remote", "list"):
		return handleRemoteList(ctx)
	case ctx.IsSubcommandPath("branch", "list"):
		return handleBranchList(ctx)
	case ctx.IsSubcommandPath("branch", "delete"):
		return handleBranchDelete(ctx)
	case ctx.IsSubcommandPath("config", "get"):
		return handleConfigGet(ctx)
	case ctx.IsSubcommandPath("config", "set"):
		return handleConfigSet(ctx)
	case ctx.IsSubcommandPath("config", "list"):
		return handleConfigList(ctx)
	default:
		return fmt.Errorf("unknown subcommand path: %v", ctx.SubcommandPath)
	}
}

func handleRemoteAdd(ctx *cf.Context) error {
	fetch := ctx.GetBool("-fetch", false)
	tags := ctx.GetBool("-tags", false)

	fmt.Printf("Adding remote repository\n")
	if fetch {
		fmt.Println("  Will fetch after adding")
	}
	if tags {
		fmt.Println("  Will import tags")
	}

	// In a real implementation, would parse positional args for name and URL
	if len(ctx.Clauses) > 0 && len(ctx.Clauses[0].Positional) >= 2 {
		name := ctx.Clauses[0].Positional[0]
		url := ctx.Clauses[0].Positional[1]
		fmt.Printf("  Name: %s\n", name)
		fmt.Printf("  URL: %s\n", url)
	}

	return nil
}

func handleRemoteRemove(ctx *cf.Context) error {
	force := ctx.GetBool("-force", false)

	fmt.Printf("Removing remote repository\n")
	if force {
		fmt.Println("  Force removal enabled")
	}

	if len(ctx.Clauses) > 0 && len(ctx.Clauses[0].Positional) >= 1 {
		name := ctx.Clauses[0].Positional[0]
		fmt.Printf("  Name: %s\n", name)
	}

	return nil
}

func handleRemoteList(ctx *cf.Context) error {
	verbose := ctx.GetBool("-verbose", false)

	fmt.Printf("Listing remote repositories\n")
	if verbose {
		fmt.Println("  origin\thttps://github.com/user/repo.git (fetch)")
		fmt.Println("  origin\thttps://github.com/user/repo.git (push)")
	} else {
		fmt.Println("  origin")
	}

	return nil
}

func handleBranchList(ctx *cf.Context) error {
	all := ctx.GetBool("-all", false)

	fmt.Printf("Listing branches\n")
	if all {
		fmt.Println("  main")
		fmt.Println("  feature-1")
		fmt.Println("  remotes/origin/main")
		fmt.Println("  remotes/origin/feature-1")
	} else {
		fmt.Println("  main")
		fmt.Println("  feature-1")
	}

	return nil
}

func handleBranchDelete(ctx *cf.Context) error {
	force := ctx.GetBool("-force", false)

	fmt.Printf("Deleting branch\n")
	if force {
		fmt.Println("  Force deletion enabled")
	}

	if len(ctx.Clauses) > 0 && len(ctx.Clauses[0].Positional) >= 1 {
		branch := ctx.Clauses[0].Positional[0]
		fmt.Printf("  Branch: %s\n", branch)
	}

	return nil
}

func handleConfigGet(ctx *cf.Context) error {
	fmt.Printf("Getting configuration value\n")

	if len(ctx.Clauses) > 0 && len(ctx.Clauses[0].Positional) >= 1 {
		key := ctx.Clauses[0].Positional[0]
		fmt.Printf("  Key: %s\n", key)
		fmt.Printf("  Value: example-value\n")
	}

	return nil
}

func handleConfigSet(ctx *cf.Context) error {
	global := ctx.GetBool("-global", false)

	fmt.Printf("Setting configuration value\n")
	if global {
		fmt.Println("  Global scope")
	} else {
		fmt.Println("  Local scope")
	}

	if len(ctx.Clauses) > 0 && len(ctx.Clauses[0].Positional) >= 2 {
		key := ctx.Clauses[0].Positional[0]
		value := ctx.Clauses[0].Positional[1]
		fmt.Printf("  Key: %s\n", key)
		fmt.Printf("  Value: %s\n", value)
	}

	return nil
}

func handleConfigList(ctx *cf.Context) error {
	fmt.Printf("Listing all configuration values\n")
	fmt.Println("  user.name=John Doe")
	fmt.Println("  user.email=john@example.com")
	fmt.Println("  core.editor=vim")

	return nil
}
