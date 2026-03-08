package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"cargo/internal/config"
	"cargo/internal/project"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage compose projects",
	Long:  "Manage cargo compose projects: list, sync, and check status.",
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured projects",
	RunE:  runProjectList,
}

var projectSyncCmd = &cobra.Command{
	Use:   "sync [name]",
	Short: "Sync one or all projects",
	Long:  "Sync a named project or all projects if no name is given.",
	RunE:  runProjectSync,
}

var projectStatusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show status of one or all projects",
	RunE:  runProjectStatus,
}

func init() {
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectSyncCmd)
	projectCmd.AddCommand(projectStatusCmd)
}

// --- list ---

func runProjectList(cmd *cobra.Command, args []string) error {
	if remoteURL != "" {
		return remoteProjectList()
	}
	return localProjectList()
}

func localProjectList() error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Projects) == 0 {
		fmt.Println("No projects configured.")
		return nil
	}

	fmt.Printf("%-20s %-40s %-15s %s\n", "NAME", "GIT URL", "REVISION", "POLL INTERVAL")
	fmt.Println(strings.Repeat("-", 90))
	for _, p := range cfg.Projects {
		poll := p.PollInterval
		if poll == "" {
			poll = "(manual)"
		}
		fmt.Printf("%-20s %-40s %-15s %s\n", p.Name, p.Git.URL, p.Git.Revision, poll)
	}
	return nil
}

func remoteProjectList() error {
	token, err := resolveToken()
	if err != nil {
		return err
	}

	body, err := doAPIRequest(http.MethodGet, remoteURL+"/api/v1/projects", token, nil)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(string(body))
		return nil
	}

	prettyPrint(result)
	return nil
}

// --- sync ---

func runProjectSync(cmd *cobra.Command, args []string) error {
	if remoteURL != "" {
		return remoteProjectSync(args)
	}
	return localProjectSync(args)
}

func localProjectSync(args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	mgr := project.NewManager(cfg)

	if len(args) == 0 {
		results := mgr.SyncAll()
		for _, r := range results {
			if r.Error != nil {
				fmt.Printf("[FAIL] %s: %v\n", r.ProjectName, r.Error)
			} else {
				fmt.Printf("[OK]   %s (commit: %s)\n", r.ProjectName, r.Commit)
			}
		}
		return nil
	}

	name := args[0]
	result := mgr.SyncProject(name)
	if result.Error != nil {
		return fmt.Errorf("syncing project %q: %w", name, result.Error)
	}
	fmt.Printf("Project %q synced successfully (commit: %s)\n", name, result.Commit)
	return nil
}

func remoteProjectSync(args []string) error {
	token, err := resolveToken()
	if err != nil {
		return err
	}

	var url string
	if len(args) == 0 {
		url = remoteURL + "/api/v1/projects/sync"
	} else {
		url = remoteURL + "/api/v1/projects/" + args[0] + "/sync"
	}

	body, err := doAPIRequest(http.MethodPost, url, token, nil)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(string(body))
		return nil
	}

	prettyPrint(result)
	return nil
}

// --- status ---

func runProjectStatus(cmd *cobra.Command, args []string) error {
	if remoteURL != "" {
		return remoteProjectStatus(args)
	}
	return localProjectStatus(args)
}

func localProjectStatus(args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	mgr := project.NewManager(cfg)

	var names []string
	if len(args) == 0 {
		for _, p := range cfg.Projects {
			names = append(names, p.Name)
		}
	} else {
		names = args
	}

	for _, name := range names {
		status, err := mgr.StatusProject(name)
		if err != nil {
			fmt.Printf("[%s] error: %v\n", name, err)
			continue
		}
		fmt.Printf("[%s]\n%s\n", name, status)
	}
	return nil
}

func remoteProjectStatus(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("project name required for remote status")
	}

	token, err := resolveToken()
	if err != nil {
		return err
	}

	url := remoteURL + "/api/v1/projects/" + args[0] + "/status"
	body, err := doAPIRequest(http.MethodGet, url, token, nil)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(string(body))
		return nil
	}

	prettyPrint(result)
	return nil
}

// --- helpers ---

// resolveToken returns the auth token from the --token flag or from the auth_token file.
func resolveToken() (string, error) {
	if authToken != "" {
		return authToken, nil
	}

	// Try to read from default location
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	tokenPath := filepath.Join(home, ".cargo", "workdir", "auth_token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("reading auth token from %q (use --token flag to specify): %w", tokenPath, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// doAPIRequest performs an HTTP request to the Cargo API with Bearer token auth.
func doAPIRequest(method, url, token string, body io.Reader) ([]byte, error) {
	// #nosec G402 - self-signed cert is expected for cargo server
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: insecureTLSConfig(),
		},
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request to %q: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// prettyPrint formats and prints a map as indented JSON.
func prettyPrint(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
