package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Manifest struct {
	SessionID  string          `json:"session_id"`
	SourceRepo string          `json:"source_repo"`
	ExportedAt string          `json:"exported_at"`
	Files      []ManifestFile  `json:"files"`
	Thread     *ThreadMetadata `json:"thread,omitempty"`
}

type ManifestFile struct {
	RelativePath string `json:"relative_path"`
	SHA256       string `json:"sha256"`
}

type ThreadMetadata struct {
	Title            string  `json:"title"`
	Preview          string  `json:"preview"`
	FirstUserMessage string  `json:"first_user_message"`
	Source           string  `json:"source"`
	ModelProvider    string  `json:"model_provider"`
	SandboxPolicy    string  `json:"sandbox_policy"`
	ApprovalMode     string  `json:"approval_mode"`
	TokensUsed       int64   `json:"tokens_used"`
	HasUserEvent     int64   `json:"has_user_event"`
	CLIVersion       string  `json:"cli_version"`
	Model            *string `json:"model,omitempty"`
	ReasoningEffort  *string `json:"reasoning_effort,omitempty"`
	HistoryMode      string  `json:"history_mode"`
	Name             *string `json:"name,omitempty"`
}

type SessionMeta struct {
	Type    string `json:"type"`
	Payload struct {
		ID  string `json:"id"`
		CWD string `json:"cwd"`
	} `json:"payload"`
}

type GenericEvent struct {
	Payload struct {
		CWD string `json:"cwd"`
	} `json:"payload"`
}

func main() {
	if len(os.Args) != 2 {
		usage()
		os.Exit(2)
	}

	var err error

	switch os.Args[1] {
	case "export":
		err = exportSession()

	case "import":
		var sid string
		var paths []string

		sid, paths, err = importSession()
		if err == nil {
			err = registerThreadInDesktop(sid, paths)
		}

	case "resume":
		err = resumeSession()

	case "status":
		err = showStatus()

	case "list":
		err = listSessions()

	case "push":
		err = pushSession()

	case "pull":
		err = pullSession()

	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "codex-sync: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`Usage:

  codex-sync export
  codex-sync import
  codex-sync resume
  codex-sync status
  codex-sync list
  codex-sync push
  codex-sync pull`)
}

func gitRoot() (string, error) {
	cmd := exec.Command(
		"git",
		"rev-parse",
		"--show-toplevel",
	)

	out, err := cmd.Output()
	if err != nil {
		return "", errors.New(
			"current directory is not inside a Git repository",
		)
	}

	root := strings.TrimSpace(string(out))

	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}

	return filepath.Clean(root), nil
}

func codexHome() (string, error) {
	if value := os.Getenv("CODEX_HOME"); value != "" {
		return filepath.Abs(value)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".codex"), nil
}

func sessionsRoot() (string, error) {
	home, err := codexHome()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "sessions"), nil
}

func syncRoot() (string, error) {
	root, err := gitRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, ".codex-sync"), nil
}

func stateDB() (string, error) {
	home, err := codexHome()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "state_5.sqlite"), nil
}

func normalizePath(value string) string {
	value = strings.TrimSpace(value)

	if strings.HasPrefix(value, `\\?\`) {
		value = value[4:]
	}

	value = strings.ReplaceAll(
		value,
		`\`,
		"/",
	)

	value = strings.TrimRight(value, "/")

	if runtime.GOOS == "windows" {
		value = strings.ToLower(value)
	}

	return value
}

func belongsToRepo(
	session string,
	repo string,
) bool {
	f, err := os.Open(session)
	if err != nil {
		return false
	}
	defer f.Close()

	want := normalizePath(repo)

	scanner := bufio.NewScanner(f)

	// Rollout lines can be large.
	scanner.Buffer(
		make([]byte, 64*1024),
		16*1024*1024,
	)

	for scanner.Scan() {
		line := scanner.Bytes()

		if !strings.Contains(
			string(line),
			`"cwd"`,
		) {
			continue
		}

		var event GenericEvent

		if err := json.Unmarshal(
			line,
			&event,
		); err != nil {
			continue
		}

		if event.Payload.CWD == "" {
			continue
		}

		if normalizePath(
			event.Payload.CWD,
		) == want {
			return true
		}
	}

	return false
}

func getSessionID(
	session string,
) (string, error) {
	f, err := os.Open(session)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	scanner.Buffer(
		make([]byte, 64*1024),
		16*1024*1024,
	)

	for scanner.Scan() {
		var event SessionMeta

		if err := json.Unmarshal(
			scanner.Bytes(),
			&event,
		); err != nil {
			continue
		}

		if event.Type != "session_meta" {
			continue
		}

		if event.Payload.ID != "" {
			return event.Payload.ID, nil
		}
	}

	return "", fmt.Errorf(
		"cannot determine session id: %s",
		session,
	)
}

func fileSHA256(
	path string,
) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()

	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(
		h.Sum(nil),
	), nil
}

type sessionFile struct {
	Path    string
	ModTime time.Time
}

func findProjectSessions(
	repo string,
) ([]sessionFile, error) {
	root, err := sessionsRoot()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf(
			"Codex sessions directory doesn't exist: %s",
			root,
		)
	}

	var matches []sessionFile

	err = filepath.WalkDir(
		root,
		func(
			path string,
			entry fs.DirEntry,
			err error,
		) error {
			if err != nil {
				return nil
			}

			if entry.IsDir() {
				return nil
			}

			if filepath.Ext(path) != ".jsonl" {
				return nil
			}

			if !belongsToRepo(path, repo) {
				return nil
			}

			info, err := entry.Info()
			if err != nil {
				return nil
			}

			matches = append(
				matches,
				sessionFile{
					Path:    path,
					ModTime: info.ModTime(),
				},
			)

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf(
			"No Codex sessions found for repository:\n%s",
			repo,
		)
	}

	sort.Slice(
		matches,
		func(i, j int) bool {
			return matches[i].
				ModTime.
				After(matches[j].ModTime)
		},
	)

	return matches, nil
}

func copyFile(
	src string,
	dst string,
) error {
	if err := os.MkdirAll(
		filepath.Dir(dst),
		0o755,
	); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err == nil {
		_ = os.Chtimes(
			dst,
			info.ModTime(),
			info.ModTime(),
		)
	}

	return nil
}

func exportSession() error {
	repo, err := gitRoot()
	if err != nil {
		return err
	}

	sessionRoot, err := sessionsRoot()
	if err != nil {
		return err
	}

	sessions, err := findProjectSessions(repo)
	if err != nil {
		return err
	}

	latest := sessions[0]

	sid, err := getSessionID(latest.Path)
	if err != nil {
		return err
	}

	var sameThread []sessionFile

	for _, item := range sessions {
		currentSID, err := getSessionID(item.Path)
		if err != nil {
			continue
		}

		if currentSID == sid {
			sameThread = append(
				sameThread,
				item,
			)
		}
	}

	if len(sameThread) == 0 {
		return fmt.Errorf(
			"no rollout files found for session %s",
			sid,
		)
	}

	targetRoot, err := syncRoot()
	if err != nil {
		return err
	}

	targetSessions := filepath.Join(
		targetRoot,
		"sessions",
	)

	if err := os.RemoveAll(
		targetSessions,
	); err != nil {
		return err
	}

	manifest := Manifest{
		SessionID:  sid,
		SourceRepo: repo,
		ExportedAt: time.Now().
			UTC().
			Format(time.RFC3339Nano),
	}

	for _, item := range sameThread {
		relative, err := filepath.Rel(
			sessionRoot,
			item.Path,
		)
		if err != nil {
			return err
		}

		target := filepath.Join(
			targetSessions,
			relative,
		)

		if err := copyFile(
			item.Path,
			target,
		); err != nil {
			return err
		}

		hash, err := fileSHA256(target)
		if err != nil {
			return err
		}

		manifest.Files = append(
			manifest.Files,
			ManifestFile{
				RelativePath: filepath.
					ToSlash(relative),
				SHA256: hash,
			},
		)
	}

	// Preserve the original UI metadata.
	meta, err := readThreadMetadata(sid)
	if err == nil {
		manifest.Thread = meta
	}

	if err := os.MkdirAll(
		targetRoot,
		0o755,
	); err != nil {
		return err
	}

	data, err := json.MarshalIndent(
		manifest,
		"",
		"  ",
	)
	if err != nil {
		return err
	}

	data = append(data, '\n')

	if err := os.WriteFile(
		filepath.Join(
			targetRoot,
			"manifest.json",
		),
		data,
		0o644,
	); err != nil {
		return err
	}

	fmt.Println("Codex session exported")
	fmt.Printf("Session: %s\n", sid)
	fmt.Printf(
		"Files:   %d\n",
		len(manifest.Files),
	)

	for _, item := range manifest.Files {
		fmt.Printf(
			"  %s\n",
			item.RelativePath,
		)
	}

	return nil
}

func readManifest() (*Manifest, error) {
	root, err := syncRoot()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(
		root,
		"manifest.json",
	)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(
				"no .codex-sync/manifest.json found; export the session on the other computer first",
			)
		}

		return nil, err
	}

	var manifest Manifest

	if err := json.Unmarshal(
		data,
		&manifest,
	); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func importSession() (
	string,
	[]string,
	error,
) {
	manifest, err := readManifest()
	if err != nil {
		return "", nil, err
	}

	if len(manifest.Files) == 0 {
		return "", nil, errors.New(
			"manifest contains no session files",
		)
	}

	sroot, err := sessionsRoot()
	if err != nil {
		return "", nil, err
	}

	sync, err := syncRoot()
	if err != nil {
		return "", nil, err
	}

	var imported []string

	for _, item := range manifest.Files {
		relative := filepath.FromSlash(
			item.RelativePath,
		)

		source := filepath.Join(
			sync,
			"sessions",
			relative,
		)

		if _, err := os.Stat(source); err != nil {
			return "", nil,
				fmt.Errorf(
					"session file missing: %s",
					source,
				)
		}

		destination := filepath.Join(
			sroot,
			relative,
		)

		if _, err := os.Stat(destination); err == nil {
			srcHash, err := fileSHA256(source)
			if err != nil {
				return "", nil, err
			}

			dstHash, err := fileSHA256(destination)
			if err != nil {
				return "", nil, err
			}

			if srcHash != dstHash {
				backup := fmt.Sprintf(
					"%s.bak-%s",
					destination,
					time.Now().
						Format(
							"20060102-150405",
						),
				)

				if err := copyFile(
					destination,
					backup,
				); err != nil {
					return "", nil, err
				}

				fmt.Printf(
					"Backup: %s\n",
					backup,
				)
			}
		}

		if err := copyFile(
			source,
			destination,
		); err != nil {
			return "", nil, err
		}

		imported = append(
			imported,
			destination,
		)
	}

	fmt.Println(
		"Codex session imported",
	)
	fmt.Printf(
		"Session: %s\n",
		manifest.SessionID,
	)
	fmt.Printf(
		"Files:   %d\n",
		len(imported),
	)

	return manifest.SessionID,
		imported,
		nil
}

func openStateDB() (
	*sql.DB,
	error,
) {
	path, err := stateDB()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	db, err := sql.Open(
		"sqlite",
		path,
	)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(
		`PRAGMA busy_timeout = 5000`,
	); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func readThreadMetadata(
	sid string,
) (*ThreadMetadata, error) {
	db, err := openStateDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var (
		meta            ThreadMetadata
		model           sql.NullString
		reasoningEffort sql.NullString
		name            sql.NullString
	)

	err = db.QueryRow(`
		SELECT
			title,
			preview,
			first_user_message,
			source,
			model_provider,
			sandbox_policy,
			approval_mode,
			tokens_used,
			has_user_event,
			cli_version,
			model,
			reasoning_effort,
			history_mode,
			name
		FROM threads
		WHERE id = ?
	`, sid).Scan(
		&meta.Title,
		&meta.Preview,
		&meta.FirstUserMessage,
		&meta.Source,
		&meta.ModelProvider,
		&meta.SandboxPolicy,
		&meta.ApprovalMode,
		&meta.TokensUsed,
		&meta.HasUserEvent,
		&meta.CLIVersion,
		&model,
		&reasoningEffort,
		&meta.HistoryMode,
		&name,
	)
	if err != nil {
		return nil, err
	}

	if model.Valid {
		meta.Model = &model.String
	}

	if reasoningEffort.Valid {
		meta.ReasoningEffort = &reasoningEffort.String
	}

	if name.Valid {
		meta.Name = &name.String
	}

	return &meta, nil
}

func ensureProject(
	db *sql.DB,
	repo string,
) (string, error) {
	// First prefer a project already associated
	// with the current checkout.
	var projectID string

	err := db.QueryRow(`
		SELECT project_id
		FROM threads
		WHERE cwd = ?
		  AND project_id IS NOT NULL
		LIMIT 1
	`, repo).Scan(&projectID)

	if err == nil && projectID != "" {
		return projectID, nil
	}

	projectName := filepath.Base(repo)

	err = db.QueryRow(`
		SELECT id
		FROM projects
		WHERE name = ?
		ORDER BY position ASC
		LIMIT 1
	`, projectName).Scan(&projectID)

	if err == nil {
		return projectID, nil
	}

	if !errors.Is(
		err,
		sql.ErrNoRows,
	) {
		return "", err
	}

	projectID = newUUID()

	var maxPosition int

	err = db.QueryRow(`
		SELECT COALESCE(MAX(position), -1)
		FROM projects
	`).Scan(&maxPosition)
	if err != nil {
		return "", err
	}

	nowMS := time.Now().
		UnixMilli()

	_, err = db.Exec(
		`
		INSERT INTO projects (
			id,
			name,
			metadata,
			position,
			created_at_ms,
			updated_at_ms
		)
		VALUES (?, ?, '{}', ?, ?, ?)
	`,
		projectID,
		projectName,
		maxPosition+1,
		nowMS,
		nowMS,
	)
	if err != nil {
		return "", err
	}

	return projectID, nil
}

func registerThreadInDesktop(
	sid string,
	importedPaths []string,
) error {
	if len(importedPaths) == 0 {
		return errors.New(
			"no imported rollout files",
		)
	}

	dbPath, err := stateDB()
	if err != nil {
		return err
	}

	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf(
				"Codex state DB not found: %s\n",
				dbPath,
			)
			fmt.Println(
				"Skipping Desktop UI registration",
			)
			return nil
		}

		return err
	}

	db, err := openStateDB()
	if err != nil {
		return err
	}
	defer db.Close()

	repo, err := gitRoot()
	if err != nil {
		return err
	}

	projectID, err := ensureProject(db, repo)
	if err != nil {
		return err
	}

	// manifest.Files are exported newest first.
	rolloutPath := importedPaths[0]

	now := time.Now().Unix()
	nowMS := time.Now().UnixMilli()

	var exists int

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM threads
		WHERE id = ?
	`, sid).Scan(&exists)
	if err != nil {
		return err
	}

	if exists > 0 {
		_, err = db.Exec(
			`
			UPDATE threads
			SET
				rollout_path = ?,
				cwd = ?,
				project_id = ?,
				archived = 0,
				updated_at = ?,
				updated_at_ms = ?,
				recency_at = ?,
				recency_at_ms = ?
			WHERE id = ?
		`,
			rolloutPath,
			repo,
			projectID,
			now,
			nowMS,
			now,
			nowMS,
			sid,
		)
		if err != nil {
			return err
		}
	} else {
		manifest, err := readManifest()
		if err != nil {
			return err
		}

		meta := manifest.Thread

		if meta == nil {
			meta = &ThreadMetadata{
				Title: fmt.Sprintf(
					"Imported Codex session %s",
					sid,
				),
				Source:        "vscode",
				ModelProvider: "openai",
				SandboxPolicy: "workspace-write",
				ApprovalMode:  "on-request",
				HasUserEvent:  1,
				HistoryMode:   "legacy",
			}

			meta.Preview = meta.Title
			meta.FirstUserMessage = meta.Title
		}

		if meta.Preview == "" {
			meta.Preview = meta.Title
		}

		if meta.FirstUserMessage == "" {
			meta.FirstUserMessage = meta.Title
		}

		if meta.Source == "" {
			meta.Source = "vscode"
		}

		if meta.ModelProvider == "" {
			meta.ModelProvider = "openai"
		}

		if meta.SandboxPolicy == "" {
			meta.SandboxPolicy = "workspace-write"
		}

		if meta.ApprovalMode == "" {
			meta.ApprovalMode = "on-request"
		}

		if meta.HistoryMode == "" {
			meta.HistoryMode = "legacy"
		}

		_, err = db.Exec(
			`
			INSERT INTO threads (
				id,
				rollout_path,
				created_at,
				updated_at,
				source,
				model_provider,
				cwd,
				title,
				sandbox_policy,
				approval_mode,
				tokens_used,
				has_user_event,
				archived,
				cli_version,
				first_user_message,
				model,
				reasoning_effort,
				preview,
				recency_at,
				recency_at_ms,
				history_mode,
				name,
				project_id
			)
			VALUES (
				?, ?, ?, ?, ?,
				?, ?, ?, ?, ?,
				?, ?, 0, ?, ?,
				?, ?, ?, ?, ?,
				?, ?, ?
			)
		`,
			sid,
			rolloutPath,
			now,
			now,
			meta.Source,
			meta.ModelProvider,
			repo,
			meta.Title,
			meta.SandboxPolicy,
			meta.ApprovalMode,
			meta.TokensUsed,
			meta.HasUserEvent,
			meta.CLIVersion,
			meta.FirstUserMessage,
			nullableString(meta.Model),
			nullableString(
				meta.ReasoningEffort,
			),
			meta.Preview,
			now,
			nowMS,
			meta.HistoryMode,
			nullableString(meta.Name),
			projectID,
		)
		if err != nil {
			return err
		}
	}

	fmt.Println(
		"Codex Desktop thread registered",
	)
	fmt.Printf(
		"Project:    %s\n",
		filepath.Base(repo),
	)
	fmt.Printf(
		"Project ID: %s\n",
		projectID,
	)

	return nil
}

func nullableString(
	value *string,
) any {
	if value == nil {
		return nil
	}

	return *value
}

func resumeSession() error {
	sid, imported, err := importSession()
	if err != nil {
		return err
	}

	if err := registerThreadInDesktop(
		sid,
		imported,
	); err != nil {
		return err
	}

	return runCodexResume(sid)
}

func runCodexResume(
	sid string,
) error {
	repo, err := gitRoot()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf(
		"Resuming %s\n",
		sid,
	)
	fmt.Println()

	cmd := exec.Command(
		"codex",
		"resume",
		sid,
	)

	cmd.Dir = repo
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func showStatus() error {
	manifest, err := readManifest()
	if err != nil {
		return err
	}

	fmt.Printf(
		"Session:  %s\n",
		manifest.SessionID,
	)
	fmt.Printf(
		"Exported: %s\n",
		manifest.ExportedAt,
	)
	fmt.Printf(
		"From:     %s\n",
		manifest.SourceRepo,
	)
	fmt.Printf(
		"Files:    %d\n",
		len(manifest.Files),
	)

	for _, item := range manifest.Files {
		fmt.Printf(
			"  %s\n",
			item.RelativePath,
		)
	}

	return nil
}

func listSessions() error {
	root, err := sessionsRoot()
	if err != nil {
		return err
	}

	var sessions []sessionFile

	err = filepath.WalkDir(
		root,
		func(
			path string,
			entry fs.DirEntry,
			err error,
		) error {
			if err != nil ||
				entry.IsDir() ||
				filepath.Ext(path) != ".jsonl" {
				return nil
			}

			info, err := entry.Info()
			if err != nil {
				return nil
			}

			sessions = append(
				sessions,
				sessionFile{
					Path:    path,
					ModTime: info.ModTime(),
				},
			)

			return nil
		},
	)
	if err != nil {
		return err
	}

	sort.Slice(
		sessions,
		func(i, j int) bool {
			return sessions[i].
				ModTime.
				After(
					sessions[j].ModTime,
				)
		},
	)

	for _, session := range sessions {
		meta, err := readSessionMeta(
			session.Path,
		)
		if err != nil {
			continue
		}

		fmt.Println()
		fmt.Printf(
			"Session: %s\n",
			meta.Payload.ID,
		)
		fmt.Printf(
			"CWD:     %s\n",
			meta.Payload.CWD,
		)
		fmt.Printf(
			"File:    %s\n",
			session.Path,
		)
	}

	return nil
}

func readSessionMeta(
	path string,
) (*SessionMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	scanner.Buffer(
		make([]byte, 64*1024),
		16*1024*1024,
	)

	for scanner.Scan() {
		var event SessionMeta

		if err := json.Unmarshal(
			scanner.Bytes(),
			&event,
		); err != nil {
			continue
		}

		if event.Type ==
			"session_meta" {
			return &event, nil
		}
	}

	return nil, errors.New(
		"session_meta not found",
	)
}

func runGit(
	args ...string,
) error {
	repo, err := gitRoot()
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"git",
		args...,
	)

	cmd.Dir = repo
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"git %s failed: %w",
			strings.Join(args, " "),
			err,
		)
	}

	return nil
}

func gitHasStagedChanges() (
	bool,
	error,
) {
	repo, err := gitRoot()
	if err != nil {
		return false, err
	}

	cmd := exec.Command(
		"git",
		"diff",
		"--cached",
		"--quiet",
	)

	cmd.Dir = repo

	err = cmd.Run()

	if err == nil {
		return false, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) &&
		exitErr.ExitCode() == 1 {
		return true, nil
	}

	return false, err
}

func pushSession() error {
	if err := exportSession(); err != nil {
		return err
	}

	if err := runGit(
		"add",
		"-A",
	); err != nil {
		return err
	}

	changed, err := gitHasStagedChanges()
	if err != nil {
		return err
	}

	if changed {
		message := fmt.Sprintf(
			"Sync Codex session %s",
			time.Now().
				Format(
					"2006-01-02 15:04",
				),
		)

		if err := runGit(
			"commit",
			"-m",
			message,
		); err != nil {
			return err
		}
	} else {
		fmt.Println(
			"No changes to commit",
		)
	}

	if err := runGit(
		"push",
	); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(
		"Codex session pushed",
	)

	return nil
}

func pullSession() error {
	if err := runGit(
		"pull",
		"--rebase",
	); err != nil {
		return err
	}

	sid, imported, err := importSession()
	if err != nil {
		return err
	}

	if err := registerThreadInDesktop(
		sid,
		imported,
	); err != nil {
		return err
	}

	return runCodexResume(sid)
}

// UUID v4 without another dependency.
func newUUID() string {
	data := make([]byte, 16)

	if _, err := rand.Read(data); err != nil {
		panic(err)
	}

	data[6] =
		(data[6] & 0x0f) | 0x40

	data[8] =
		(data[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		data[0:4],
		data[4:6],
		data[6:8],
		data[8:10],
		data[10:16],
	)
}
