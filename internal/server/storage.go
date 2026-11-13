package server

import (
	"context"
	"github.com/YangTaeyoung/folio/internal/blob"
	"net/http"
	"sort"
	"strings"
	"time"
)

func (s *Server) storage(w http.ResponseWriter, r *http.Request, p principal) error {
	if e := s.require(r, p, true, ""); e != nil {
		return e
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/storage"), "/")
	if len(parts) == 1 && r.Method == "GET" {
		list := []Backend{}
		for _, b := range s.st.Backends {
			list = append(list, b.Backend)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.Before(list[j].CreatedAt) })
		writeJSON(w, 200, map[string]any{"backends": list})
		return nil
	}
	if len(parts) == 1 && r.Method == "POST" {
		var in struct{ Name, Provider, Bucket, Region, Endpoint, Prefix, AccountName, AccessKeyID, SecretAccessKey, SessionToken, ServiceAccountJSON, ConnectionString, AccountKey string }
		if e := decode(r, &in); e != nil {
			return e
		}
		if strings.TrimSpace(in.Name) == "" {
			return fail(400, "required_field", "Storage name is required")
		}
		if in.Provider != "local" && in.Provider != "s3" && in.Provider != "gcs" && in.Provider != "azure" {
			return fail(400, "invalid_provider", "Unsupported storage provider")
		}
		b := Backend{ID: random(), Name: in.Name, Provider: in.Provider, Bucket: in.Bucket, Region: in.Region, Endpoint: in.Endpoint, Prefix: in.Prefix, AccountName: in.AccountName, CreatedAt: now(), CredentialsConfigured: in.AccessKeyID != "" || in.SecretAccessKey != "" || in.ServiceAccountJSON != "" || in.ConnectionString != "" || in.AccountKey != ""}
		c := blob.Config{ID: b.ID, Name: b.Name, Provider: b.Provider, Root: s.cfg.DataDir + "/objects/" + b.ID, Bucket: b.Bucket, Region: b.Region, Endpoint: b.Endpoint, Prefix: b.Prefix, AccountName: b.AccountName, AccessKeyID: in.AccessKeyID, SecretAccessKey: in.SecretAccessKey, SessionToken: in.SessionToken, ServiceAccountJSON: in.ServiceAccountJSON, ConnectionString: in.ConnectionString, AccountKey: in.AccountKey}
		if _, e := s.factory(c); e != nil {
			return fail(400, "invalid_storage_config", "Storage configuration is invalid")
		}
		encrypted, e := s.encrypt(c)
		if e != nil {
			return e
		}
		if e = s.mutate(func() error {
			s.st.Backends[b.ID] = backendRecord{b, encrypted}
			s.audit(p, "storage.created", b.ID, b.Provider)
			return nil
		}); e != nil {
			return e
		}
		writeJSON(w, 201, map[string]any{"backend": b})
		return nil
	}
	if len(parts) < 2 {
		return fail(404, "not_found", "Not found")
	}
	id := parts[1]
	b, ok := s.st.Backends[id]
	if !ok {
		return fail(404, "not_found", "Storage backend not found")
	}
	if len(parts) == 3 && parts[2] == "test" && r.Method == "POST" {
		store, e := s.store(id)
		if e != nil {
			return fail(400, "invalid_storage_config", "Storage configuration is invalid")
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		if e = store.Test(ctx); e != nil {
			s.audit(p, "storage.test_failed", id, "")
			s.save()
			writeJSON(w, 200, map[string]any{"ok": false, "message": "Storage probe failed; verify credentials, permissions, endpoint and bucket"})
			return nil
		}
		s.audit(p, "storage.tested", id, "")
		if e = s.save(); e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"ok": true, "message": "Write, read and delete probe succeeded"})
		return nil
	}
	if len(parts) == 3 && parts[2] == "default" && r.Method == "POST" {
		if e := s.mutate(func() error {
			for key, v := range s.st.Backends {
				v.IsDefault = key == id
				s.st.Backends[key] = v
			}
			s.audit(p, "storage.default_changed", id, "")
			return nil
		}); e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"backend": s.st.Backends[id].Backend})
		return nil
	}
	if len(parts) == 2 && r.Method == "DELETE" {
		if s.pendingBackends[id] > 0 {
			return fail(409, "backend_busy", "An upload is using this backend")
		}
		if b.IsDefault {
			return fail(409, "default_backend", "Select another default before deleting this backend")
		}
		for _, list := range s.st.Versions {
			for _, v := range list {
				if v.BackendID == id {
					return fail(409, "backend_referenced", "Retained versions reference this backend")
				}
			}
		}
		for _, g := range s.st.Garbage {
			if g.BackendID == id {
				return fail(409, "backend_referenced", "Pending object cleanup references this backend")
			}
		}
		if e := s.mutate(func() error { delete(s.st.Backends, id); s.audit(p, "storage.deleted", id, ""); return nil }); e != nil {
			return e
		}
		w.WriteHeader(204)
		return nil
	}
	return fail(404, "not_found", "Not found")
}
