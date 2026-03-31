package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/AfeefRazick/coda-cli/internal/api"
	"github.com/spf13/cobra"
)

// ── types ────────────────────────────────────────────────────────────────────

type listResponse struct {
	Items         []json.RawMessage `json:"items"`
	NextPageToken string            `json:"nextPageToken"`
	NextPageLink  string            `json:"nextPageLink"`
	NextSyncToken string            `json:"nextSyncToken"`
	Href          string            `json:"href"`
}

type codaDoc struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Owner       string `json:"owner"`
	OwnerName   string `json:"ownerName"`
	BrowserLink string `json:"browserLink"`
	UpdatedAt   string `json:"updatedAt"`
	Workspace   struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"workspace"`
	Folder struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"folder"`
}

type codaPage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Subtitle    string `json:"subtitle"`
	BrowserLink string `json:"browserLink"`
	Type        string `json:"type"`
	Parent      struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"parent"`
}

type codaTable struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	BrowserLink string `json:"browserLink"`
	RowCount    int    `json:"rowCount"`
	Href        string `json:"href"`
}

type codaColumn struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Display     bool   `json:"display"`
	Calculated  bool   `json:"calculated"`
	Formula     string `json:"formula"`
	BrowserLink string `json:"browserLink"`
}

type codaUser struct {
	Name      string `json:"name"`
	LoginId   string `json:"loginId"`
	Type      string `json:"type"`
	TokenName string `json:"tokenName"`
	Href      string `json:"href"`
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string  { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error { *s = append(*s, v); return nil }
func (s *stringSliceFlag) Type() string    { return "key=value" }

// ── flag helpers ─────────────────────────────────────────────────────────────

func addWaitFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("wait", false, "Block until the async mutation completes")
	cmd.Flags().Duration("timeout", 2*time.Minute, "Maximum time to wait before giving up (requires --wait)")
	cmd.Flags().Duration("interval", 2*time.Second, "How often to poll mutation status (requires --wait)")
}

func addConfirmFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("yes", false, "Skip the interactive confirmation prompt")
}

func addPageCreateFlags(cmd *cobra.Command) {
	cmd.Flags().String("name", "", "Name of the page")
	cmd.Flags().String("subtitle", "", "Subtitle shown below the page name")
	cmd.Flags().String("icon", "", "Icon name for the page")
	cmd.Flags().String("image-url", "", "URL of the page cover image")
	cmd.Flags().String("parent-page-id", "", "ID of the parent page (creates a subpage)")
	cmd.Flags().String("content", "", "Initial canvas content (HTML by default; see --content-format)")
	cmd.Flags().String("content-format", "html", "Format of --content: html or markdown")
	cmd.Flags().String("embed-url", "", "URL to embed as the page content (cannot be combined with --content)")
}

func addPageUpdateFlags(cmd *cobra.Command) {
	cmd.Flags().String("name", "", "New name for the page")
	cmd.Flags().String("subtitle", "", "New subtitle for the page")
	cmd.Flags().String("icon", "", "New icon name for the page")
	cmd.Flags().String("image-url", "", "New cover image URL")
	cmd.Flags().Bool("hidden", false, "Hide the page from the doc navigation")
	cmd.Flags().String("content", "", "Canvas content to write (see --insertion-mode and --content-format)")
	cmd.Flags().String("content-format", "html", "Format of --content: html or markdown")
	cmd.Flags().String("insertion-mode", "replace", "How content is written: replace, append, or prepend")
	cmd.Flags().String("element-id", "", "Page element ID to use as the insertion reference point")
}

func addRowEditFlags(cmd *cobra.Command) {
	cmd.Flags().StringArray("value", nil, "Cell value as column=value (repeatable); numbers/booleans/JSON are parsed automatically")
	cmd.Flags().Bool("disable-parsing", false, "Treat all --value values as literal strings, skipping type inference")
}

// ── query / payload builders ─────────────────────────────────────────────────

func addStringFlag(cmd *cobra.Command, query url.Values, name string) {
	value, _ := cmd.Flags().GetString(name)
	if value != "" {
		query.Set(name, value)
	}
}

func addBoolFlag(cmd *cobra.Command, query url.Values, flagName, queryName string) {
	if cmd.Flags().Changed(flagName) {
		value, _ := cmd.Flags().GetBool(flagName)
		query.Set(queryName, strconv.FormatBool(value))
	}
}

func copyStringQueryFlag(cmd *cobra.Command, query url.Values, flagName string, queryName ...string) {
	name := flagName
	if len(queryName) > 0 {
		name = queryName[0]
	}
	value, err := cmd.Flags().GetString(flagName)
	if err == nil && value != "" {
		query.Set(name, value)
		return
	}
	if intValue, err := cmd.Flags().GetInt(flagName); err == nil && cmd.Flags().Changed(flagName) {
		query.Set(name, strconv.Itoa(intValue))
	}
}

func copyStringFlag(cmd *cobra.Command, payload map[string]any, flagName string, bodyName ...string) {
	name := flagName
	if len(bodyName) > 0 {
		name = bodyName[0]
	}
	value, _ := cmd.Flags().GetString(flagName)
	if value != "" {
		payload[name] = value
	}
}

func pageCreatePayload(cmd *cobra.Command) (map[string]any, error) {
	payload := map[string]any{}
	copyStringFlag(cmd, payload, "name")
	copyStringFlag(cmd, payload, "subtitle")
	copyStringFlag(cmd, payload, "icon", "iconName")
	copyStringFlag(cmd, payload, "image-url", "imageUrl")
	copyStringFlag(cmd, payload, "parent-page-id", "parentPageId")
	content, _ := cmd.Flags().GetString("content")
	embedURL, _ := cmd.Flags().GetString("embed-url")
	if content != "" && embedURL != "" {
		return nil, errors.New("use either --content or --embed-url, not both")
	}
	if content != "" {
		format, _ := cmd.Flags().GetString("content-format")
		payload["pageContent"] = canvasContent(content, format)
	}
	if embedURL != "" {
		payload["pageContent"] = map[string]any{"type": "embed", "url": embedURL}
	}
	if len(payload) == 0 {
		return nil, errors.New("no page fields provided")
	}
	return payload, nil
}

func pageUpdatePayload(cmd *cobra.Command) (map[string]any, error) {
	payload := map[string]any{}
	copyStringFlag(cmd, payload, "name")
	copyStringFlag(cmd, payload, "subtitle")
	copyStringFlag(cmd, payload, "icon", "iconName")
	copyStringFlag(cmd, payload, "image-url", "imageUrl")
	if cmd.Flags().Changed("hidden") {
		hidden, _ := cmd.Flags().GetBool("hidden")
		payload["isHidden"] = hidden
	}
	content, _ := cmd.Flags().GetString("content")
	if content != "" {
		format, _ := cmd.Flags().GetString("content-format")
		mode, _ := cmd.Flags().GetString("insertion-mode")
		elementID, _ := cmd.Flags().GetString("element-id")
		contentUpdate := map[string]any{
			"insertionMode": mode,
			"canvasContent": map[string]any{
				"format":  format,
				"content": content,
			},
		}
		if elementID != "" {
			contentUpdate["elementId"] = elementID
		}
		payload["contentUpdate"] = contentUpdate
	}
	if len(payload) == 0 {
		return nil, errors.New("no update fields provided")
	}
	return payload, nil
}

func pageDeleteContentPayload(cmd *cobra.Command) (map[string]any, error) {
	elementIDs, err := cmd.Flags().GetStringArray("element-id")
	if err != nil {
		return nil, err
	}
	if len(elementIDs) == 0 {
		return nil, nil
	}
	return map[string]any{"elementIds": elementIDs}, nil
}

func docCreatePayload(cmd *cobra.Command) (map[string]any, error) {
	payload := map[string]any{}
	copyStringFlag(cmd, payload, "title")
	copyStringFlag(cmd, payload, "source-doc", "sourceDoc")
	copyStringFlag(cmd, payload, "timezone")
	copyStringFlag(cmd, payload, "folder", "folderId")
	content, _ := cmd.Flags().GetString("content")
	pageName, _ := cmd.Flags().GetString("page-name")
	pageSubtitle, _ := cmd.Flags().GetString("page-subtitle")
	if pageName != "" || pageSubtitle != "" || content != "" {
		initial := map[string]any{}
		if pageName != "" {
			initial["name"] = pageName
		}
		if pageSubtitle != "" {
			initial["subtitle"] = pageSubtitle
		}
		if content != "" {
			initial["pageContent"] = canvasContent(content, "html")
		}
		payload["initialPage"] = initial
	}
	if len(payload) == 0 {
		return nil, errors.New("no doc fields provided; use flags or --input")
	}
	return payload, nil
}

func docUpdatePayload(cmd *cobra.Command) (map[string]any, error) {
	payload := map[string]any{}
	copyStringFlag(cmd, payload, "title")
	copyStringFlag(cmd, payload, "icon", "iconName")
	if len(payload) == 0 {
		return nil, errors.New("no update fields provided; use flags or --input")
	}
	return payload, nil
}

func rowsUpsertPayload(cmd *cobra.Command, keys []string) (map[string]any, error) {
	input, _ := cmd.Flags().GetString("input")
	if input != "" {
		payload, err := readJSONInput(input)
		if err != nil {
			return nil, err
		}
		obj, ok := payload.(map[string]any)
		if !ok {
			return nil, errors.New("input JSON must be an object")
		}
		if len(keys) > 0 {
			if _, exists := obj["keyColumns"]; !exists {
				obj["keyColumns"] = keys
			}
		}
		return obj, nil
	}
	cells, err := parseCellEdits(cmd)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"rows": []any{map[string]any{"cells": cells}}}
	if len(keys) > 0 {
		payload["keyColumns"] = keys
	}
	return payload, nil
}

func rowUpdatePayload(cmd *cobra.Command) (map[string]any, error) {
	input, _ := cmd.Flags().GetString("input")
	if input != "" {
		payload, err := readJSONInput(input)
		if err != nil {
			return nil, err
		}
		obj, ok := payload.(map[string]any)
		if !ok {
			return nil, errors.New("input JSON must be an object")
		}
		return obj, nil
	}
	cells, err := parseCellEdits(cmd)
	if err != nil {
		return nil, err
	}
	return map[string]any{"row": map[string]any{"cells": cells}}, nil
}

func rowsDeletePayload(cmd *cobra.Command) (map[string]any, error) {
	input, _ := cmd.Flags().GetString("input")
	if input != "" {
		payload, err := readJSONInput(input)
		if err != nil {
			return nil, err
		}
		obj, ok := payload.(map[string]any)
		if !ok {
			return nil, errors.New("input JSON must be an object")
		}
		return obj, nil
	}
	rowIDs, err := cmd.Flags().GetStringArray("row")
	if err != nil {
		return nil, err
	}
	if len(rowIDs) == 0 {
		return nil, errors.New("at least one --row is required unless --input is used")
	}
	return map[string]any{"rowIds": rowIDs}, nil
}

func parseCellEdits(cmd *cobra.Command) ([]map[string]any, error) {
	values, err := cmd.Flags().GetStringArray("value")
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, errors.New("at least one --value is required")
	}
	cells := make([]map[string]any, 0, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid cell edit %q, expected column=value", value)
		}
		cells = append(cells, map[string]any{
			"column": strings.TrimSpace(parts[0]),
			"value":  parseScalar(strings.TrimSpace(parts[1])),
		})
	}
	return cells, nil
}

func canvasContent(content, format string) map[string]any {
	return map[string]any{
		"type": "canvas",
		"canvasContent": map[string]any{
			"format":  format,
			"content": content,
		},
	}
}

func inputOrPayload(cmd *cobra.Command, build func(*cobra.Command) (map[string]any, error)) (map[string]any, error) {
	input, _ := cmd.Flags().GetString("input")
	if input != "" {
		payload, err := readJSONInput(input)
		if err != nil {
			return nil, err
		}
		obj, ok := payload.(map[string]any)
		if !ok {
			return nil, errors.New("input JSON must be an object")
		}
		return obj, nil
	}
	return build(cmd)
}

func bodyFromFlags(fields []string, input string) (any, error) {
	if input != "" && len(fields) > 0 {
		return nil, errors.New("use either --input or --field")
	}
	if input != "" {
		data, err := os.ReadFile(filepath.Clean(input))
		if err != nil {
			return nil, fmt.Errorf("failed to read input file: %w", err)
		}
		var body any
		if err := json.Unmarshal(data, &body); err != nil {
			return nil, fmt.Errorf("failed to parse input JSON: %w", err)
		}
		return body, nil
	}
	if len(fields) == 0 {
		return nil, nil
	}
	body := map[string]any{}
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid field %q, expected key=value", field)
		}
		body[strings.TrimSpace(parts[0])] = parseScalar(strings.TrimSpace(parts[1]))
	}
	return body, nil
}

func readJSONInput(path string) (any, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read input file: %w", err)
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse input JSON: %w", err)
	}
	return payload, nil
}

func parseScalar(value string) any {
	if strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") {
		var parsed any
		if err := json.Unmarshal([]byte(value), &parsed); err == nil {
			return parsed
		}
	}
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}
	if value == "null" {
		return nil
	}
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	return value
}

// ── run helpers ───────────────────────────────────────────────────────────────

func runSimpleGet(ctx context.Context, path string) error {
	client, _, err := api.NewClient()
	if err != nil {
		return err
	}
	body, _, _, err := client.Request(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	return printJSON(body)
}

func runDelete(cmd *cobra.Command, path, kind, id string) error {
	if err := confirmDestructive(cmd, kind, id); err != nil {
		return err
	}
	client, _, err := api.NewClient()
	if err != nil {
		return err
	}
	body, _, _, err := client.Request(cmd.Context(), http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	return maybeWaitAndPrint(cmd, client, body)
}

func maybeWaitAndPrint(cmd *cobra.Command, client *api.Client, body []byte) error {
	wait, _ := cmd.Flags().GetBool("wait")
	if !wait {
		return printJSON(body)
	}
	requestID := mutationRequestID(body)
	if requestID == "" {
		return printJSON(body)
	}
	timeout, _ := cmd.Flags().GetDuration("timeout")
	interval, _ := cmd.Flags().GetDuration("interval")
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()
	status, err := client.WaitForMutation(ctx, requestID, interval)
	if err != nil {
		return err
	}
	return printJSONMarshal(map[string]any{
		"request": status,
		"initial": json.RawMessage(body),
	})
}

func paginateItems(ctx context.Context, client *api.Client, path string, initialQuery url.Values) ([]json.RawMessage, listResponse, error) {
	query := cloneValues(initialQuery)
	var allItems []json.RawMessage
	var last listResponse

	for {
		body, _, _, err := client.Request(ctx, http.MethodGet, path, query, nil)
		if err != nil {
			return nil, listResponse{}, err
		}
		if err := json.Unmarshal(body, &last); err != nil {
			return nil, listResponse{}, fmt.Errorf("failed to parse paginated response: %w", err)
		}
		allItems = append(allItems, last.Items...)
		if last.NextPageToken == "" {
			break
		}
		if query == nil {
			query = url.Values{}
		}
		query.Set("pageToken", last.NextPageToken)
	}

	return allItems, last, nil
}

func cloneValues(values url.Values) url.Values {
	if values == nil {
		return nil
	}
	cloned := url.Values{}
	for key, list := range values {
		for _, value := range list {
			cloned.Add(key, value)
		}
	}
	return cloned
}

func mutationRequestID(body []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	for _, key := range []string{"requestId", "id"} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func exactArgsFor(usage string, count int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == count {
			return nil
		}
		return fmt.Errorf("expected %d argument(s) for `%s`; got %d\n\nUsage:\n  %s", count, usage, len(args), usage)
	}
}

func confirmDestructive(cmd *cobra.Command, kind, id string) error {
	yes, _ := cmd.Flags().GetBool("yes")
	if yes {
		return nil
	}

	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		return fmt.Errorf("refusing to delete %s %q without --yes in non-interactive mode", kind, id)
	}

	fmt.Printf("Delete %s %q? Type 'yes' to continue: ", kind, id)
	var response string
	_, err = fmt.Scanln(&response)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}
	if strings.TrimSpace(strings.ToLower(response)) != "yes" {
		return errors.New("deletion cancelled")
	}
	return nil
}

func promptForToken() (string, error) {
	fmt.Print("Coda API token: ")
	var token string
	_, err := fmt.Scanln(&token)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("failed to read token: %w", err)
	}
	return strings.TrimSpace(token), nil
}

// ── output helpers ────────────────────────────────────────────────────────────

func printJSON(body []byte) error {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Println(string(body))
		return nil
	}
	return printJSONMarshal(payload)
}

func printJSONMarshal(payload any) error {
	pretty, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to render JSON: %w", err)
	}
	fmt.Println(string(pretty))
	return nil
}

func printDocTableFromBody(body []byte) error {
	var resp listResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	return printDocTable(resp.Items)
}

func printDocTable(items []json.RawMessage) error {
	docs := make([]codaDoc, 0, len(items))
	for _, item := range items {
		var doc codaDoc
		if err := json.Unmarshal(item, &doc); err != nil {
			return err
		}
		docs = append(docs, doc)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tWORKSPACE\tFOLDER\tOWNER")
	for _, doc := range docs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", doc.ID, doc.Name, doc.Workspace.Name, doc.Folder.Name, firstNonEmpty(doc.OwnerName, doc.Owner))
	}
	return w.Flush()
}

func printPageTableFromBody(body []byte) error {
	var resp listResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	return printPageTable(resp.Items)
}

func printPageTable(items []json.RawMessage) error {
	pages := make([]codaPage, 0, len(items))
	for _, item := range items {
		var page codaPage
		if err := json.Unmarshal(item, &page); err != nil {
			return err
		}
		pages = append(pages, page)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSUBTITLE\tPARENT")
	for _, page := range pages {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", page.ID, page.Name, page.Subtitle, page.Parent.Name)
	}
	return w.Flush()
}

func printTableTableFromBody(body []byte) error {
	var resp listResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	tables := make([]codaTable, 0, len(resp.Items))
	for _, item := range resp.Items {
		var table codaTable
		if err := json.Unmarshal(item, &table); err != nil {
			return err
		}
		tables = append(tables, table)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTYPE\tROWS")
	for _, table := range tables {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", table.ID, table.Name, table.Type, table.RowCount)
	}
	return w.Flush()
}

func printColumnTableFromBody(body []byte) error {
	var resp listResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	columns := make([]codaColumn, 0, len(resp.Items))
	for _, item := range resp.Items {
		var column codaColumn
		if err := json.Unmarshal(item, &column); err != nil {
			return err
		}
		columns = append(columns, column)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTYPE\tDISPLAY\tCALCULATED")
	for _, column := range columns {
		fmt.Fprintf(w, "%s\t%s\t%s\t%t\t%t\n", column.ID, column.Name, column.Type, column.Display, column.Calculated)
	}
	return w.Flush()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func escapeSegment(value string) string {
	return url.PathEscape(value)
}
