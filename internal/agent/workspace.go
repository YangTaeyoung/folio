package agent

import (
	"context"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/url"
)

type CreationContext struct {
	Agent      string   `json:"agent,omitempty"`
	Model      string   `json:"model,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Repository string   `json:"repository,omitempty"`
	Commit     string   `json:"commit,omitempty"`
	TaskURL    string   `json:"taskUrl,omitempty"`
	Sources    []string `json:"sources,omitempty"`
}
type FeedbackInput struct {
	ID        string `json:"id"`
	VersionID string `json:"versionId"`
}
type CommentInput struct {
	ID        string `json:"id"`
	VersionID string `json:"versionId"`
	ParentID  string `json:"parentId,omitempty"`
	Body      string `json:"body"`
	Path      string `json:"path,omitempty"`
	Anchor    string `json:"anchor,omitempty"`
}
type ResolveInput struct {
	ID        string `json:"id"`
	CommentID string `json:"commentId"`
	Resolved  bool   `json:"resolved"`
}
type ReviewInput struct {
	ID        string `json:"id"`
	VersionID string `json:"versionId"`
	Status    string `json:"status" jsonschema:"draft, in_review, approved, changes_requested. Only mark approved when the user actually authorized approval."`
}
type PromoteInput struct {
	ID                     string `json:"id"`
	VersionID              string `json:"versionId"`
	BasePublishedVersionID string `json:"basePublishedVersionId" jsonschema:"Official version ID observed from inspect, empty only if never published"`
}
type ReviewersInput struct {
	ID          string   `json:"id"`
	ReviewerIDs []string `json:"reviewerIds"`
}
type DiffInput struct {
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
	Path string `json:"path,omitempty" jsonschema:"Optional exact file path to include bounded text line differences"`
}
type ContextInput struct {
	ID         string           `json:"id"`
	Tags       *[]string        `json:"tags,omitempty"`
	Provenance *CreationContext `json:"provenance,omitempty"`
}
type CollectionInput struct {
	ID            string   `json:"id,omitempty" jsonschema:"Omit to create, supply to update"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Visibility    string   `json:"visibility" jsonschema:"private or organization. Does not change artifact sharing."`
	ArtifactIDs   []string `json:"artifactIds" jsonschema:"Ordered readable artifact IDs"`
	BaseUpdatedAt string   `json:"baseUpdatedAt,omitempty"`
}
type GuestInput struct {
	ID        string `json:"id"`
	VersionID string `json:"versionId"`
	ExpiresAt string `json:"expiresAt" jsonschema:"Required RFC3339 expiry, at most 365 days"`
	Password  string `json:"password,omitempty" jsonschema:"Optional password. Do not echo or store it in artifact context."`
}
type GuestRevokeInput struct {
	ID     string `json:"id"`
	LinkID string `json:"linkId"`
}
type TrashInput struct {
	ID     string `json:"id"`
	Action string `json:"action" jsonschema:"trash or restore. Permanent purge is deliberately not an agent operation."`
}

func addWorkspaceTools(s *mcp.Server, c *Client, tool func(string, string, bool) *mcp.Tool) {
	endpoint := func(id string) string { return "/api/artifacts/" + url.PathEscape(id) }
	mcp.AddTool(s, tool("folio_feedback", "Read exact-version review state and feedback threads before revising. Read access required; draft versions require explicit reviewer or owner permission.", true), func(ctx context.Context, _ *mcp.CallToolRequest, in FeedbackInput) (*mcp.CallToolResult, map[string]any, error) {
		q := url.Values{"versionId": {in.VersionID}}
		comments, e := c.JSON(ctx, "GET", endpoint(in.ID)+"/comments?"+q.Encode(), nil)
		if e != nil {
			return nil, nil, e
		}
		review, e := c.JSON(ctx, "GET", endpoint(in.ID)+"/review?"+q.Encode(), nil)
		return nil, map[string]any{"comments": comments["comments"], "review": review["review"]}, e
	})
	mcp.AddTool(s, tool("folio_comment", "Add exact-version feedback or reply to a root thread. Requires review or publish token scope and readable version.", false), func(ctx context.Context, _ *mcp.CallToolRequest, in CommentInput) (*mcp.CallToolResult, map[string]any, error) {
		v, e := c.JSON(ctx, "POST", endpoint(in.ID)+"/comments", map[string]any{"versionId": in.VersionID, "parentId": in.ParentID, "body": in.Body, "path": in.Path, "anchor": in.Anchor})
		return nil, v, e
	})
	mcp.AddTool(s, tool("folio_resolve_thread", "Resolve or reopen a feedback thread you authored or own. Requires review or publish scope.", false), func(ctx context.Context, _ *mcp.CallToolRequest, in ResolveInput) (*mcp.CallToolResult, map[string]any, error) {
		v, e := c.JSON(ctx, "PATCH", endpoint(in.ID)+"/comments/"+url.PathEscape(in.CommentID), map[string]bool{"resolved": in.Resolved})
		return nil, v, e
	})
	mcp.AddTool(s, tool("folio_review", "Set exact-version review state. Request review after revising; approval must reflect an explicit human decision. Requires review or publish scope.", false), func(ctx context.Context, _ *mcp.CallToolRequest, in ReviewInput) (*mcp.CallToolResult, map[string]any, error) {
		v, e := c.JSON(ctx, "PUT", endpoint(in.ID)+"/review", map[string]string{"versionId": in.VersionID, "status": in.Status})
		return nil, v, e
	})
	mcp.AddTool(s, tool("folio_promote", "Make one approved version official without changing the latest working draft. Requires owner/admin, publish scope, approved review and matching official base.", false), func(ctx context.Context, _ *mcp.CallToolRequest, in PromoteInput) (*mcp.CallToolResult, map[string]any, error) {
		v, e := c.JSON(ctx, "POST", endpoint(in.ID)+"/promote", map[string]string{"versionId": in.VersionID, "basePublishedVersionId": in.BasePublishedVersionID})
		return nil, v, e
	})
	mcp.AddTool(s, tool("folio_reviewers", "Replace explicit draft reviewer IDs. Reviewers must also have normal artifact sharing permission. Requires owner/admin and share scope.", false), func(ctx context.Context, _ *mcp.CallToolRequest, in ReviewersInput) (*mcp.CallToolResult, map[string]any, error) {
		v, e := c.JSON(ctx, "PATCH", endpoint(in.ID)+"/reviewers", map[string]any{"reviewerIds": in.ReviewerIDs})
		return nil, v, e
	})
	mcp.AddTool(s, tool("folio_compare_versions", "Compare authorized retained versions: real SHA256 file changes, and optional text line diff (256KiB/1000 lines per side).", true), func(ctx context.Context, _ *mcp.CallToolRequest, in DiffInput) (*mcp.CallToolResult, map[string]any, error) {
		q := url.Values{"from": {in.From}, "to": {in.To}, "path": {in.Path}}
		v, e := c.JSON(ctx, "GET", endpoint(in.ID)+"/diff?"+q.Encode(), nil)
		return nil, v, e
	})
	mcp.AddTool(s, tool("folio_set_context", "Set artifact tags and explicit provenance for future version snapshots. Retained version snapshots remain unchanged. Do not include secrets.", false), func(ctx context.Context, _ *mcp.CallToolRequest, in ContextInput) (*mcp.CallToolResult, map[string]any, error) {
		body := map[string]any{}
		if in.Tags != nil {
			body["tags"] = in.Tags
		}
		if in.Provenance != nil {
			body["provenance"] = in.Provenance
		}
		v, e := c.JSON(ctx, "PATCH", endpoint(in.ID)+"/metadata", body)
		return nil, v, e
	})
	mcp.AddTool(s, tool("folio_collections", "List accessible project collections. Item lists are individually filtered by artifact permission.", true), func(ctx context.Context, _ *mcp.CallToolRequest, _ Empty) (*mcp.CallToolResult, map[string]any, error) {
		v, e := c.JSON(ctx, "GET", "/api/collections", nil)
		return nil, v, e
	})
	mcp.AddTool(s, tool("folio_save_collection", "Create/update an ordered project collection. Organization visibility NEVER grants access to its artifacts. Requires publish scope.", false), func(ctx context.Context, _ *mcp.CallToolRequest, in CollectionInput) (*mcp.CallToolResult, map[string]any, error) {
		method, path := "POST", "/api/collections"
		if in.ID != "" {
			method = "PATCH"
			path += "/" + url.PathEscape(in.ID)
		}
		v, e := c.JSON(ctx, method, path, map[string]any{"name": in.Name, "description": in.Description, "visibility": in.Visibility, "artifactIds": in.ArtifactIDs, "baseUpdatedAt": in.BaseUpdatedAt})
		return nil, v, e
	})
	mcp.AddTool(s, tool("folio_guest_links", "List version-pinned guest links you manage, including expiry/revocation, never password hashes.", true), func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, map[string]any, error) {
		v, e := c.JSON(ctx, "GET", endpoint(in.ID)+"/guest-links", nil)
		return nil, v, e
	})
	mcp.AddTool(s, tool("folio_create_guest_link", "Explicitly grant read-only guest access to one version until expiry. A draft may be shared explicitly. Requires owner/admin and share scope. Password optional.", false), func(ctx context.Context, _ *mcp.CallToolRequest, in GuestInput) (*mcp.CallToolResult, map[string]any, error) {
		v, e := c.JSON(ctx, "POST", endpoint(in.ID)+"/guest-links", map[string]string{"versionId": in.VersionID, "expiresAt": in.ExpiresAt, "password": in.Password})
		return nil, v, e
	})
	mcp.AddTool(s, tool("folio_revoke_guest_link", "Revoke a guest link and all its current content sessions immediately. Requires owner/admin and share scope.", false), func(ctx context.Context, _ *mcp.CallToolRequest, in GuestRevokeInput) (*mcp.CallToolResult, map[string]any, error) {
		v, e := c.JSON(ctx, "POST", endpoint(in.ID)+"/guest-links/"+url.PathEscape(in.LinkID)+"/revoke", Empty{})
		return nil, v, e
	})
	mcp.AddTool(s, tool("folio_trash", "Move an artifact into recoverable 30-day trash, or restore it. Requires owner/admin and publish scope. Does not permanently purge.", false), func(ctx context.Context, _ *mcp.CallToolRequest, in TrashInput) (*mcp.CallToolResult, map[string]any, error) {
		method, path := "DELETE", endpoint(in.ID)
		if in.Action == "restore" {
			method = "POST"
			path = "/api/trash/" + url.PathEscape(in.ID) + "/restore"
		} else if in.Action != "trash" {
			return nil, nil, errors.New("action must be trash or restore")
		}
		v, e := c.JSON(ctx, method, path, Empty{})
		return nil, v, e
	})
}
