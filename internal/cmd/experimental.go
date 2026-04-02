package cmd

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/AfeefRazick/coda-cli/internal/auth"
	"github.com/spf13/cobra"
)

const experimentalIndexVersion = 1

type experimentalDoc struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Owner     string `json:"owner"`
	OwnerName string `json:"ownerName"`
	UpdatedAt string `json:"updatedAt"`
	CreatedAt string `json:"createdAt"`
	Workspace struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"workspace"`
	Folder struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"folder"`
}

type experimentalPage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Subtitle    string `json:"subtitle"`
	Type        string `json:"type"`
	Href        string `json:"href"`
	BrowserLink string `json:"browserLink"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	Parent      struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"parent"`
}

type experimentalIndexState struct {
	Version     int                            `json:"version"`
	IndexedAt   string                         `json:"indexedAt"`
	Docs        map[string]experimentalDocMeta `json:"docs"`
	ContentRoot string                         `json:"contentRoot"`
}

type experimentalDocMeta struct {
	ID          string                          `json:"id"`
	Name        string                          `json:"name"`
	Owner       string                          `json:"owner"`
	OwnerName   string                          `json:"ownerName"`
	WorkspaceID string                          `json:"workspaceId"`
	Workspace   string                          `json:"workspace"`
	FolderID    string                          `json:"folderId"`
	Folder      string                          `json:"folder"`
	CreatedAt   string                          `json:"createdAt"`
	UpdatedAt   string                          `json:"updatedAt"`
	Pages       map[string]experimentalPageMeta `json:"pages"`
}

type experimentalPageMeta struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Subtitle     string `json:"subtitle"`
	Type         string `json:"type"`
	ParentID     string `json:"parentId"`
	ParentName   string `json:"parentName"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	BrowserLink  string `json:"browserLink"`
	RelativePath string `json:"relativePath"`
	ContentHash  string `json:"contentHash"`
	IndexedAt    string `json:"indexedAt"`
}

type experimentalIndexStats struct {
	DocsAdded      int `json:"docsAdded"`
	DocsUpdated    int `json:"docsUpdated"`
	DocsDeleted    int `json:"docsDeleted"`
	DocsUnchanged  int `json:"docsUnchanged"`
	PagesAdded     int `json:"pagesAdded"`
	PagesUpdated   int `json:"pagesUpdated"`
	PagesDeleted   int `json:"pagesDeleted"`
	PagesUnchanged int `json:"pagesUnchanged"`
}

type experimentalGrepMatch struct {
	DocID        string `json:"docId"`
	DocName      string `json:"docName"`
	PageID       string `json:"pageId"`
	PageName     string `json:"pageName"`
	ParentName   string `json:"parentName"`
	Line         int    `json:"line"`
	Text         string `json:"text"`
	RelativePath string `json:"relativePath"`
}

func newExperimentalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experimental",
		Short: "Experimental local indexing and search",
		Long:  "Experimental commands for building a local snapshot of docs/pages and searching that snapshot without a server-side grep API.",
	}

	indexCmd := &cobra.Command{
		Use:   "index",
		Short: "Build or refresh a local workspace index",
		Long:  "Fetch docs, pages, and page content into a local cache, then refresh only changed docs on later runs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := api.NewClient()
			if err != nil {
				return err
			}
			workers, _ := cmd.Flags().GetInt("workers")
			jsonOut, _ := cmd.Flags().GetBool("json")
			force, _ := cmd.Flags().GetBool("force")
			result, err := runExperimentalIndex(cmd.Context(), client, workers, force)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSONMarshal(result)
			}
			fmt.Printf("Indexed %d docs at %s\n", len(result.State.Docs), result.State.IndexedAt)
			fmt.Printf("Docs: +%d ~%d -%d =%d\n", result.Stats.DocsAdded, result.Stats.DocsUpdated, result.Stats.DocsDeleted, result.Stats.DocsUnchanged)
			fmt.Printf("Pages: +%d ~%d -%d =%d\n", result.Stats.PagesAdded, result.Stats.PagesUpdated, result.Stats.PagesDeleted, result.Stats.PagesUnchanged)
			fmt.Printf("Index path: %s\n", result.IndexDir)
			return nil
		},
	}
	indexCmd.Flags().Int("workers", 6, "Number of concurrent page-content fetches for changed docs")
	indexCmd.Flags().Bool("force", false, "Re-fetch all docs even if their updatedAt matches the local index")
	indexCmd.Flags().Bool("json", false, "Print the index summary as JSON")
	cmd.AddCommand(indexCmd)

	grepCmd := &cobra.Command{
		Use:   "grep <pattern>",
		Short: "Search the local experimental index",
		Long:  "Run a local grep-like search over the content fetched by `coda experimental index`.",
		Args:  exactArgsFor("coda experimental grep <pattern>", 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			matches, err := runExperimentalGrep(cmd, args[0])
			if err != nil {
				return err
			}
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				return printJSONMarshal(map[string]any{"matches": matches, "count": len(matches)})
			}
			for _, match := range matches {
				location := match.DocName + " / " + match.PageName
				if match.ParentName != "" {
					location += " (parent: " + match.ParentName + ")"
				}
				fmt.Printf("%s:%d:%s\n", location, match.Line, match.Text)
			}
			if len(matches) == 0 {
				fmt.Println("No matches found.")
			}
			return nil
		},
	}
	grepCmd.Flags().Bool("ignore-case", false, "Match case-insensitively")
	grepCmd.Flags().Bool("fixed-strings", false, "Treat the pattern as a literal string instead of a regular expression")
	grepCmd.Flags().String("doc", "", "Only search docs whose name or ID contains this value")
	grepCmd.Flags().String("page", "", "Only search pages whose name or ID contains this value")
	grepCmd.Flags().Int("limit", 100, "Maximum number of matches to print")
	grepCmd.Flags().Bool("json", false, "Print matches as JSON")
	cmd.AddCommand(grepCmd)

	return cmd
}

type experimentalIndexResult struct {
	IndexDir string                 `json:"indexDir"`
	State    experimentalIndexState `json:"state"`
	Stats    experimentalIndexStats `json:"stats"`
}

func runExperimentalIndex(ctx context.Context, client *api.Client, workers int, force bool) (*experimentalIndexResult, error) {
	if workers < 1 {
		workers = 1
	}
	indexDir, err := experimentalIndexDir()
	if err != nil {
		return nil, err
	}
	contentRoot := filepath.Join(indexDir, "docs")
	if err := os.MkdirAll(contentRoot, 0700); err != nil {
		return nil, fmt.Errorf("failed to create index dir: %w", err)
	}

	statePath := filepath.Join(indexDir, "state.json")
	previous, err := readExperimentalState(statePath)
	if err != nil {
		return nil, err
	}

	docs, err := listExperimentalDocs(ctx, client)
	if err != nil {
		return nil, err
	}

	next := experimentalIndexState{
		Version:     experimentalIndexVersion,
		IndexedAt:   time.Now().UTC().Format(time.RFC3339),
		Docs:        make(map[string]experimentalDocMeta, len(docs)),
		ContentRoot: contentRoot,
	}
	stats := experimentalIndexStats{}
	seenDocs := make(map[string]struct{}, len(docs))

	for _, doc := range docs {
		seenDocs[doc.ID] = struct{}{}
		priorDoc, hadPrior := previous.Docs[doc.ID]
		if hadPrior && priorDoc.UpdatedAt == doc.UpdatedAt && !force {
			next.Docs[doc.ID] = priorDoc
			stats.DocsUnchanged++
			stats.PagesUnchanged += len(priorDoc.Pages)
			continue
		}
		if hadPrior {
			stats.DocsUpdated++
		} else {
			stats.DocsAdded++
		}

		pages, err := listExperimentalPages(ctx, client, doc.ID)
		if err != nil {
			return nil, err
		}
		pageMeta, pageStats, err := indexExperimentalPages(ctx, client, workers, contentRoot, doc, pages, priorDoc.Pages)
		if err != nil {
			return nil, err
		}
		stats.PagesAdded += pageStats.PagesAdded
		stats.PagesUpdated += pageStats.PagesUpdated
		stats.PagesDeleted += pageStats.PagesDeleted
		stats.PagesUnchanged += pageStats.PagesUnchanged

		next.Docs[doc.ID] = experimentalDocMeta{
			ID:          doc.ID,
			Name:        doc.Name,
			Owner:       doc.Owner,
			OwnerName:   doc.OwnerName,
			WorkspaceID: doc.Workspace.ID,
			Workspace:   doc.Workspace.Name,
			FolderID:    doc.Folder.ID,
			Folder:      doc.Folder.Name,
			CreatedAt:   doc.CreatedAt,
			UpdatedAt:   doc.UpdatedAt,
			Pages:       pageMeta,
		}
	}

	for docID, doc := range previous.Docs {
		if _, ok := seenDocs[docID]; ok {
			continue
		}
		stats.DocsDeleted++
		stats.PagesDeleted += len(doc.Pages)
		if err := os.RemoveAll(filepath.Join(contentRoot, docID)); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to remove deleted doc %s: %w", docID, err)
		}
	}

	if err := writeExperimentalState(statePath, next); err != nil {
		return nil, err
	}

	return &experimentalIndexResult{IndexDir: indexDir, State: next, Stats: stats}, nil
}

func indexExperimentalPages(ctx context.Context, client *api.Client, workers int, contentRoot string, doc experimentalDoc, pages []experimentalPage, previous map[string]experimentalPageMeta) (map[string]experimentalPageMeta, experimentalIndexStats, error) {
	if previous == nil {
		previous = map[string]experimentalPageMeta{}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	next := make(map[string]experimentalPageMeta, len(pages))
	stats := experimentalIndexStats{}
	seen := make(map[string]struct{}, len(pages))

	type result struct {
		page experimentalPage
		text string
		err  error
	}

	jobs := make(chan experimentalPage)
	results := make(chan result, len(pages))
	var workerWG sync.WaitGroup
	for range workers {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for page := range jobs {
				text, err := fetchExperimentalPageText(ctx, client, doc.ID, page.ID)
				results <- result{page: page, text: text, err: err}
			}
		}()
	}
	go func() {
		for _, page := range pages {
			jobs <- page
		}
		close(jobs)
		workerWG.Wait()
		close(results)
	}()

	for res := range results {
		if res.err != nil {
			return nil, experimentalIndexStats{}, res.err
		}
		page := res.page
		seen[page.ID] = struct{}{}
		relativePath := filepath.Join(doc.ID, safeExperimentalFileName(page.Name)+"-"+page.ID+".txt")
		fullPath := filepath.Join(contentRoot, relativePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
			return nil, experimentalIndexStats{}, fmt.Errorf("failed to create page dir: %w", err)
		}
		if err := os.WriteFile(fullPath, []byte(res.text), 0600); err != nil {
			return nil, experimentalIndexStats{}, fmt.Errorf("failed to write page content: %w", err)
		}
		hash := sha256.Sum256([]byte(res.text))
		meta := experimentalPageMeta{
			ID:           page.ID,
			Name:         page.Name,
			Subtitle:     page.Subtitle,
			Type:         page.Type,
			ParentID:     page.Parent.ID,
			ParentName:   page.Parent.Name,
			CreatedAt:    page.CreatedAt,
			UpdatedAt:    page.UpdatedAt,
			BrowserLink:  page.BrowserLink,
			RelativePath: relativePath,
			ContentHash:  hex.EncodeToString(hash[:]),
			IndexedAt:    now,
		}
		if prior, ok := previous[page.ID]; ok {
			if prior.ContentHash == meta.ContentHash && prior.Name == meta.Name && prior.Subtitle == meta.Subtitle && prior.ParentID == meta.ParentID {
				stats.PagesUnchanged++
			} else {
				stats.PagesUpdated++
			}
			if prior.RelativePath != meta.RelativePath {
				_ = os.Remove(filepath.Join(contentRoot, prior.RelativePath))
			}
		} else {
			stats.PagesAdded++
		}
		next[page.ID] = meta
	}

	for pageID, page := range previous {
		if _, ok := seen[pageID]; ok {
			continue
		}
		stats.PagesDeleted++
		if page.RelativePath != "" {
			if err := os.Remove(filepath.Join(contentRoot, page.RelativePath)); err != nil && !os.IsNotExist(err) {
				return nil, experimentalIndexStats{}, fmt.Errorf("failed to remove deleted page %s: %w", pageID, err)
			}
		}
	}

	return next, stats, nil
}

func fetchExperimentalPageText(ctx context.Context, client *api.Client, docID, pageID string) (string, error) {
	path := "/docs/" + escapeSegment(docID) + "/pages/" + escapeSegment(pageID) + "/content"
	body, err := requestExperimentalPageContent(ctx, client, path)
	if err != nil {
		return "", err
	}
	text := normalizeExperimentalContent(body)
	if strings.TrimSpace(text) == "" {
		return "", nil
	}
	return text, nil
}

func requestExperimentalPageContent(ctx context.Context, client *api.Client, path string) ([]byte, error) {
	query := url.Values{"contentFormat": {"plainText"}}
	for attempt := 0; attempt < 6; attempt++ {
		body, headers, _, err := client.Request(ctx, http.MethodGet, path, query, nil)
		if err == nil {
			return body, nil
		}

		var apiErr *api.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != http.StatusTooManyRequests {
			return nil, err
		}

		wait := experimentalRetryAfter(headers.Get("Retry-After"), attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}

	return nil, fmt.Errorf("page content request kept hitting rate limits: %s", path)
}

func experimentalRetryAfter(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration((attempt+1)*2) * time.Second
}

func normalizeExperimentalContent(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return trimmed
	}
	parts := collectExperimentalStrings(payload, "")
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func collectExperimentalStrings(value any, key string) []string {
	switch typed := value.(type) {
	case map[string]any:
		preferred := []string{"text", "content", "value", "name", "subtitle"}
		parts := make([]string, 0)
		seen := map[string]struct{}{}
		for _, name := range preferred {
			child, ok := typed[name]
			if !ok {
				continue
			}
			for _, part := range collectExperimentalStrings(child, name) {
				if _, exists := seen[part]; exists || strings.TrimSpace(part) == "" {
					continue
				}
				seen[part] = struct{}{}
				parts = append(parts, part)
			}
		}
		keys := make([]string, 0, len(typed))
		for childKey := range typed {
			keys = append(keys, childKey)
		}
		sort.Strings(keys)
		for _, childKey := range keys {
			if shouldSkipExperimentalField(childKey) {
				continue
			}
			for _, part := range collectExperimentalStrings(typed[childKey], childKey) {
				if _, exists := seen[part]; exists || strings.TrimSpace(part) == "" {
					continue
				}
				seen[part] = struct{}{}
				parts = append(parts, part)
			}
		}
		return parts
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, collectExperimentalStrings(item, key)...)
		}
		return parts
	case string:
		value := strings.TrimSpace(typed)
		if value == "" || looksLikeExperimentalMetadata(key, value) {
			return nil
		}
		return []string{value}
	default:
		return nil
	}
}

func looksLikeExperimentalMetadata(key, value string) bool {
	if shouldSkipExperimentalField(key) {
		return true
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return true
	}
	return false
}

func shouldSkipExperimentalField(key string) bool {
	switch key {
	case "id", "href", "browserLink", "type", "createdAt", "updatedAt", "parent", "parentPageId", "icon", "format":
		return true
	default:
		return false
	}
}

func listExperimentalDocs(ctx context.Context, client *api.Client) ([]experimentalDoc, error) {
	items, _, err := paginateItems(ctx, client, "/docs", url.Values{"limit": {"100"}})
	if err != nil {
		return nil, err
	}
	docs := make([]experimentalDoc, 0, len(items))
	for _, item := range items {
		var doc experimentalDoc
		if err := json.Unmarshal(item, &doc); err != nil {
			return nil, fmt.Errorf("failed to parse doc metadata: %w", err)
		}
		docs = append(docs, doc)
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Name < docs[j].Name
	})
	return docs, nil
}

func listExperimentalPages(ctx context.Context, client *api.Client, docID string) ([]experimentalPage, error) {
	items, _, err := paginateItems(ctx, client, "/docs/"+escapeSegment(docID)+"/pages", url.Values{"limit": {"100"}})
	if err != nil {
		return nil, err
	}
	pages := make([]experimentalPage, 0, len(items))
	for _, item := range items {
		var page experimentalPage
		if err := json.Unmarshal(item, &page); err != nil {
			return nil, fmt.Errorf("failed to parse page metadata: %w", err)
		}
		pages = append(pages, page)
	}
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].Name < pages[j].Name
	})
	return pages, nil
}

func runExperimentalGrep(cmd *cobra.Command, rawPattern string) ([]experimentalGrepMatch, error) {
	state, indexDir, err := loadExperimentalIndex()
	if err != nil {
		return nil, err
	}
	ignoreCase, _ := cmd.Flags().GetBool("ignore-case")
	fixed, _ := cmd.Flags().GetBool("fixed-strings")
	docFilter, _ := cmd.Flags().GetString("doc")
	pageFilter, _ := cmd.Flags().GetString("page")
	limit, _ := cmd.Flags().GetInt("limit")
	matcher, err := compileExperimentalMatcher(rawPattern, fixed, ignoreCase)
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 1
	}

	matches := make([]experimentalGrepMatch, 0)
	for _, docID := range sortedExperimentalDocIDs(state.Docs) {
		doc := state.Docs[docID]
		if !experimentalMatchFilter(docFilter, doc.ID, doc.Name) {
			continue
		}
		for _, pageID := range sortedExperimentalPageIDs(doc.Pages) {
			page := doc.Pages[pageID]
			if !experimentalMatchFilter(pageFilter, page.ID, page.Name) {
				continue
			}
			fullPath := filepath.Join(indexDir, "docs", page.RelativePath)
			fileMatches, err := grepExperimentalFile(fullPath, doc, page, matcher, limit-len(matches))
			if err != nil {
				return nil, err
			}
			matches = append(matches, fileMatches...)
			if len(matches) >= limit {
				return matches, nil
			}
		}
	}
	return matches, nil
}

func grepExperimentalFile(path string, doc experimentalDocMeta, page experimentalPageMeta, matcher func(string) bool, remaining int) ([]experimentalGrepMatch, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open indexed page %s: %w", page.ID, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	lineNo := 0
	matches := make([]experimentalGrepMatch, 0)
	for scanner.Scan() {
		lineNo++
		text := scanner.Text()
		if !matcher(text) {
			continue
		}
		matches = append(matches, experimentalGrepMatch{
			DocID:        doc.ID,
			DocName:      doc.Name,
			PageID:       page.ID,
			PageName:     page.Name,
			ParentName:   page.ParentName,
			Line:         lineNo,
			Text:         text,
			RelativePath: page.RelativePath,
		})
		if len(matches) >= remaining {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read indexed page %s: %w", page.ID, err)
	}
	return matches, nil
}

func compileExperimentalMatcher(pattern string, fixed, ignoreCase bool) (func(string) bool, error) {
	if fixed {
		needle := pattern
		if ignoreCase {
			needle = strings.ToLower(needle)
			return func(line string) bool {
				return strings.Contains(strings.ToLower(line), needle)
			}, nil
		}
		return func(line string) bool {
			return strings.Contains(line, needle)
		}, nil
	}
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regexp: %w", err)
	}
	return re.MatchString, nil
}

func readExperimentalState(path string) (experimentalIndexState, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return experimentalIndexState{Version: experimentalIndexVersion, Docs: map[string]experimentalDocMeta{}}, nil
		}
		return experimentalIndexState{}, fmt.Errorf("failed to read index state: %w", err)
	}
	var state experimentalIndexState
	if err := json.Unmarshal(data, &state); err != nil {
		return experimentalIndexState{}, fmt.Errorf("failed to parse index state: %w", err)
	}
	if state.Docs == nil {
		state.Docs = map[string]experimentalDocMeta{}
	}
	for docID, doc := range state.Docs {
		if doc.Pages == nil {
			doc.Pages = map[string]experimentalPageMeta{}
			state.Docs[docID] = doc
		}
	}
	return state, nil
}

func writeExperimentalState(path string, state experimentalIndexState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode index state: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write index state: %w", err)
	}
	return nil
}

func loadExperimentalIndex() (experimentalIndexState, string, error) {
	indexDir, err := experimentalIndexDir()
	if err != nil {
		return experimentalIndexState{}, "", err
	}
	state, err := readExperimentalState(filepath.Join(indexDir, "state.json"))
	if err != nil {
		return experimentalIndexState{}, "", err
	}
	if len(state.Docs) == 0 {
		return experimentalIndexState{}, "", fmt.Errorf("no local experimental index found; run 'coda experimental index' first")
	}
	return state, indexDir, nil
}

func experimentalIndexDir() (string, error) {
	configDir, err := auth.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "experimental", "workspace-index"), nil
}

func safeExperimentalFileName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return "page"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if lastDash {
			continue
		}
		b.WriteByte('-')
		lastDash = true
	}
	value := strings.Trim(b.String(), "-")
	if value == "" {
		return "page"
	}
	return value
}

func sortedExperimentalDocIDs(docs map[string]experimentalDocMeta) []string {
	ids := make([]string, 0, len(docs))
	for id := range docs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedExperimentalPageIDs(pages map[string]experimentalPageMeta) []string {
	ids := make([]string, 0, len(pages))
	for id := range pages {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func experimentalMatchFilter(filter string, values ...string) bool {
	filter = strings.TrimSpace(strings.ToLower(filter))
	if filter == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), filter) {
			return true
		}
	}
	return false
}
