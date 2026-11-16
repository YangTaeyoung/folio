package server

// The portable format is a size-limited, uncompressed ZIP encrypted in 1 MiB
// AES-256-GCM records. A scrypt-derived key and random salt protect the password;
// a mandatory authenticated terminal record detects truncated/reordered streams.
// References: https://pkg.go.dev/golang.org/x/crypto/scrypt#Key
// https://pkg.go.dev/crypto/cipher#AEAD ; https://pkg.go.dev/archive/zip#Reader
// Provider secrets remain encrypted in the snapshot but are intentionally NOT
// imported: restore preserves the destination's auth/DNS/provider configuration.
import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/YangTaeyoung/folio/internal/blob"
	"golang.org/x/crypto/scrypt"
)

const backupMaxBytes int64 = 512 << 20
const backupStateMax int64 = 16 << 20
const backupMaxObjects = 50000
const backupChunk = 1 << 20
const backupMagic = "FOLIOB1\n"
const backupScope = "All metadata and retained version objects, including remote objects. Restore uses new local storage; current users, authentication, Google, DNS and provider configuration are preserved. Imported provider secrets, API tokens, guest links and ephemeral guest/viewer sessions are not activated. Current administrator sessions remain active."

type backupObject struct {
	BackendID, Key, Entry, SHA256 string
	Size                          int64
}
type backupManifest struct {
	Format    int
	CreatedAt time.Time
	State     json.RawMessage
	Objects   []backupObject
}

func backupIDOK(id string) bool {
	if len(id) != 64 {
		return false
	}
	_, e := hex.DecodeString(id)
	return e == nil
}
func (s *Server) backupDir() string { return filepath.Join(s.cfg.DataDir, "backups") }
func (s *Server) backupBusy() bool {
	for _, n := range s.inFlight {
		if n > 0 {
			return true
		}
	}
	for _, n := range s.pendingBackends {
		if n > 0 {
			return true
		}
	}
	return len(s.pendingUploads) > 0
}
func backupPassword(p string) error {
	if len(p) < 12 || len(p) > 256 {
		return fail(400, "weak_password", "Backup password must contain between 12 and 256 bytes")
	}
	return nil
}
func backupCipher(password string, salt []byte) (cipher.AEAD, error) {
	key, e := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if e != nil {
		return nil, e
	}
	b, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(b)
}
func backupNonce(prefix []byte, index uint32) []byte {
	n := make([]byte, 12)
	copy(n, prefix)
	binary.BigEndian.PutUint32(n[8:], index)
	return n
}
func encryptBackup(dst io.Writer, src io.Reader, password string) error {
	salt := make([]byte, 16)
	prefix := make([]byte, 8)
	if _, e := rand.Read(salt); e != nil {
		return e
	}
	if _, e := rand.Read(prefix); e != nil {
		return e
	}
	header := append(append([]byte(backupMagic), salt...), prefix...)
	if _, e := dst.Write(header); e != nil {
		return e
	}
	g, e := backupCipher(password, salt)
	if e != nil {
		return e
	}
	buf := make([]byte, backupChunk)
	var index uint32
	for {
		n, re := io.ReadFull(src, buf)
		if re != nil && re != io.EOF && re != io.ErrUnexpectedEOF {
			return re
		}
		sealed := g.Seal(nil, backupNonce(prefix, index), buf[:n], header)
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(n))
		if _, e = dst.Write(length[:]); e != nil {
			return e
		}
		if _, e = dst.Write(sealed); e != nil {
			return e
		}
		index++
		if n == 0 {
			return nil
		}
		if index > uint32(backupMaxBytes/backupChunk+2) {
			return errors.New("backup too large")
		}
	}
}
func decryptBackup(dst io.Writer, src io.Reader, password string) error {
	header := make([]byte, len(backupMagic)+24)
	if _, e := io.ReadFull(src, header); e != nil {
		return errors.New("invalid backup")
	}
	if string(header[:len(backupMagic)]) != backupMagic {
		return errors.New("invalid backup")
	}
	salt := header[len(backupMagic) : len(backupMagic)+16]
	prefix := header[len(backupMagic)+16:]
	g, e := backupCipher(password, salt)
	if e != nil {
		return e
	}
	var total int64
	for index := uint32(0); ; index++ {
		var length [4]byte
		if _, e = io.ReadFull(src, length[:]); e != nil {
			return errors.New("backup truncated")
		}
		n := binary.BigEndian.Uint32(length[:])
		if n > backupChunk {
			return errors.New("invalid backup record")
		}
		total += int64(n)
		if total > backupMaxBytes {
			return errors.New("backup too large")
		}
		sealed := make([]byte, int(n)+g.Overhead())
		if _, e = io.ReadFull(src, sealed); e != nil {
			return errors.New("backup truncated")
		}
		plain, e := g.Open(nil, backupNonce(prefix, index), sealed, header)
		if e != nil {
			return errors.New("wrong password or modified backup")
		}
		if n == 0 {
			var extra [1]byte
			if n, e = readBackupTail(src, extra[:]); n != 0 || e != io.EOF {
				return errors.New("unexpected backup data")
			}
			return nil
		}
		if _, e = dst.Write(plain); e != nil {
			return e
		}
	}
}
func readBackupTail(r io.Reader, p []byte) (uint32, error) { n, e := r.Read(p); return uint32(n), e }

func (s *Server) backups(w http.ResponseWriter, r *http.Request, p principal) error {
	if e := s.require(r, p, true, ""); e != nil {
		return e
	}
	suffix := strings.TrimPrefix(r.URL.Path, "/api/backups")
	if suffix == "" && r.Method == "GET" {
		entries, e := os.ReadDir(s.backupDir())
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		list := []BackupInfo{}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".folio") {
				continue
			}
			id := strings.TrimSuffix(entry.Name(), ".folio")
			if !backupIDOK(id) {
				continue
			}
			info, e := entry.Info()
			if e != nil || !info.Mode().IsRegular() {
				continue
			}
			list = append(list, BackupInfo{ID: id, CreatedAt: info.ModTime().UTC(), SizeBytes: info.Size(), Scope: backupScope})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
		writeJSON(w, 200, map[string]any{"backups": list, "scope": backupScope, "maxBytes": backupMaxBytes})
		return nil
	}
	if suffix == "" && r.Method == "POST" {
		var in struct{ Password string }
		if e := decode(r, &in); e != nil {
			return e
		}
		if e := backupPassword(in.Password); e != nil {
			return e
		}
		if s.backupBusy() {
			return fail(409, "storage_busy", "Wait for active uploads before creating a backup")
		}
		info, e := s.createBackup(r.Context(), in.Password, p)
		if e != nil {
			return e
		}
		writeJSON(w, 201, map[string]any{"backup": info})
		return nil
	}
	if suffix == "/restore" && r.Method == "POST" {
		if s.backupBusy() {
			return fail(409, "storage_busy", "Wait for active uploads before restoring a backup")
		}
		// The body is raw .folio data; its password is a request header, never a URL.
		password := r.Header.Get("X-Backup-Password")
		if encoding := r.Header.Get("X-Backup-Password-Encoding"); encoding != "" {
			if encoding != "base64" {
				return fail(400, "invalid_backup", "Unsupported password encoding")
			}
			raw, e := base64.StdEncoding.DecodeString(password)
			if e != nil {
				return fail(400, "invalid_backup", "Invalid password encoding")
			}
			password = string(raw)
		}
		if e := backupPassword(password); e != nil {
			return e
		}
		r.Body = http.MaxBytesReader(w, r.Body, backupMaxBytes+(1<<20))
		if e := s.restoreBackup(r.Context(), r.Body, password, p); e != nil {
			return fail(400, "invalid_backup", "Backup could not be validated or restored")
		}
		writeJSON(w, 200, map[string]any{"restored": true, "scope": backupScope})
		return nil
	}
	if strings.HasPrefix(suffix, "/") && r.Method == "GET" {
		id := strings.TrimPrefix(suffix, "/")
		if !backupIDOK(id) {
			return fail(404, "not_found", "Backup not found")
		}
		f, e := os.Open(filepath.Join(s.backupDir(), id+".folio"))
		if e != nil {
			return fail(404, "not_found", "Backup not found")
		}
		defer f.Close()
		info, e := f.Stat()
		if e != nil || !info.Mode().IsRegular() {
			return fail(404, "not_found", "Backup not found")
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="folio-`+id+`.folio"`)
		w.Header().Set("Cache-Control", "no-store")
		http.ServeContent(w, r, "folio-"+id+".folio", info.ModTime(), f)
		return nil
	}
	return fail(404, "not_found", "Backup not found")
}

func (s *Server) createBackup(ctx context.Context, password string, p principal) (BackupInfo, error) {
	var result BackupInfo
	if e := os.MkdirAll(s.backupDir(), 0700); e != nil {
		return result, e
	}
	stage, e := os.CreateTemp(s.backupDir(), ".snapshot-*")
	if e != nil {
		return result, e
	}
	defer os.Remove(stage.Name())
	defer stage.Close()
	// State is serialized under the request's server mutex. No upload can publish
	// between manifest construction and object copying.
	stateRaw, e := json.Marshal(s.st)
	if e != nil {
		return result, e
	}
	if int64(len(stateRaw)) > backupStateMax {
		return result, fail(413, "backup_too_large", "Metadata exceeds the portable backup limit")
	}
	manifest := backupManifest{Format: 1, CreatedAt: now(), State: stateRaw, Objects: []backupObject{}}
	zw := zip.NewWriter(stage)
	var total int64
	seen := map[string]bool{}
	for _, versions := range s.st.Versions {
		for _, v := range versions {
			for _, f := range v.Files {
				identity := v.BackendID + "\x00" + f.Key
				if seen[identity] {
					continue
				}
				seen[identity] = true
				if len(manifest.Objects) >= backupMaxObjects || f.Size < 0 || f.Size > backupMaxBytes-total {
					zw.Close()
					return result, fail(413, "backup_too_large", "Objects exceed the portable backup limit")
				}
				total += f.Size
				st, e := s.store(v.BackendID)
				if e != nil {
					zw.Close()
					return result, fail(502, "storage_failed", "A retained object is unavailable")
				}
				rd, e := st.Open(ctx, f.Key)
				if e != nil {
					zw.Close()
					return result, fail(502, "storage_failed", "A retained object is unavailable")
				}
				entry := fmt.Sprintf("objects/%06d", len(manifest.Objects))
				out, e := zw.CreateHeader(&zip.FileHeader{Name: entry, Method: zip.Store})
				if e != nil {
					rd.Close()
					zw.Close()
					return result, e
				}
				hash := sha256.New()
				n, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(rd, f.Size+1))
				closeErr := rd.Close()
				if copyErr != nil || closeErr != nil || n != f.Size {
					zw.Close()
					return result, fail(502, "storage_failed", "A retained object has an invalid size")
				}
				manifest.Objects = append(manifest.Objects, backupObject{BackendID: v.BackendID, Key: f.Key, Entry: entry, Size: f.Size, SHA256: hex.EncodeToString(hash.Sum(nil))})
			}
		}
	}
	raw, e := json.Marshal(manifest)
	if e != nil {
		zw.Close()
		return result, e
	}
	if int64(len(raw)) > backupStateMax*2 {
		zw.Close()
		return result, fail(413, "backup_too_large", "Manifest exceeds the portable backup limit")
	}
	out, e := zw.CreateHeader(&zip.FileHeader{Name: "manifest.json", Method: zip.Store})
	if e == nil {
		_, e = out.Write(raw)
	}
	if closeErr := zw.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		return result, e
	}
	pos, e := stage.Seek(0, io.SeekEnd)
	if e != nil || pos > backupMaxBytes {
		return result, fail(413, "backup_too_large", "Portable backup exceeds its size limit")
	}
	if _, e = stage.Seek(0, io.SeekStart); e != nil {
		return result, e
	}
	encrypted, e := os.CreateTemp(s.backupDir(), ".encrypted-*")
	if e != nil {
		return result, e
	}
	defer encrypted.Close()
	defer os.Remove(encrypted.Name())
	if e = encryptBackup(encrypted, stage, password); e != nil {
		return result, e
	}
	if e = encrypted.Sync(); e != nil {
		return result, e
	}
	stat, e := encrypted.Stat()
	if e != nil {
		return result, e
	}
	if e = encrypted.Close(); e != nil {
		return result, e
	}
	id := digest(random())
	if e = os.Rename(encrypted.Name(), filepath.Join(s.backupDir(), id+".folio")); e != nil {
		return result, e
	}
	if e = s.mutate(func() error { s.audit(p, "backup.created", id, ""); return nil }); e != nil {
		os.Remove(filepath.Join(s.backupDir(), id+".folio"))
		return result, e
	}
	return BackupInfo{ID: id, CreatedAt: manifest.CreatedAt, SizeBytes: stat.Size(), Scope: backupScope}, nil
}

func (s *Server) restoreBackup(ctx context.Context, src io.Reader, password string, p principal) error {
	if s.backupBusy() {
		return errors.New("uploads active")
	}
	if p.user == nil || p.user.Role != "admin" {
		return errors.New("administrator required")
	}
	if e := os.MkdirAll(s.backupDir(), 0700); e != nil {
		return e
	}
	stage, e := os.CreateTemp(s.backupDir(), ".restore-*")
	if e != nil {
		return e
	}
	defer stage.Close()
	defer os.Remove(stage.Name())
	if e = decryptBackup(stage, io.LimitReader(src, backupMaxBytes+(1<<20)), password); e != nil {
		return e
	}
	size, e := stage.Seek(0, io.SeekEnd)
	if e != nil {
		return e
	}
	zr, e := zip.NewReader(stage, size)
	if e != nil {
		return e
	}
	manifest, files, e := validateBackup(zr)
	if e != nil {
		return e
	}
	var imported state
	if e = json.Unmarshal(manifest.State, &imported); e != nil {
		return e
	}
	if e = validateBackupState(imported, manifest); e != nil {
		return e
	}
	// Stage only into a fresh opaque local backend, never source/provider keys.
	id := random()
	root := filepath.Join(s.cfg.DataDir, "objects", id)
	st, e := blob.New(blob.Config{ID: id, Provider: "local", Root: root})
	if e != nil {
		return e
	}
	committed := false
	defer func() {
		if !committed {
			if closer, ok := st.(io.Closer); ok {
				closer.Close()
			}
			os.RemoveAll(root)
		}
	}()
	remap := map[string]string{}
	for _, obj := range manifest.Objects {
		if e = ctx.Err(); e != nil {
			return e
		}
		rd, e := files[obj.Entry].Open()
		if e != nil {
			return e
		}
		key := random()
		hash := sha256.New()
		e = st.Put(ctx, key, io.TeeReader(rd, hash), obj.Size, "application/octet-stream")
		closeErr := rd.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
		if hex.EncodeToString(hash.Sum(nil)) != obj.SHA256 {
			return errors.New("object checksum mismatch")
		}
		remap[obj.BackendID+"\x00"+obj.Key] = key
	}
	for aid, versions := range imported.Versions {
		for vi := range versions {
			v := &versions[vi]
			for fi := range v.Files {
				v.Files[fi].Key = remap[v.BackendID+"\x00"+v.Files[fi].Key]
			}
			v.BackendID = id
		}
		imported.Versions[aid] = versions
	}
	// Current identity and recovery settings always win, including disabled state.
	if imported.Users == nil {
		imported.Users = map[string]userRecord{}
	}
	if e := mergeRestoreIdentities(&imported, s.st.Users); e != nil {
		return e
	}
	imported.Sessions = s.st.Sessions
	imported.Tokens = map[string]tokenRecord{}
	imported.GuestSessions = map[string]GuestSession{}
	imported.Grants = map[string]Viewer{}
	imported.Viewers = map[string]Viewer{}
	imported.Garbage = []Garbage{}
	imported.GoogleAuth = s.st.GoogleAuth
	imported.GoogleSubjects = s.st.GoogleSubjects
	imported.GoogleHostedDomains = s.st.GoogleHostedDomains
	imported.DNSSecret = s.st.DNSSecret
	imported.Domains = s.st.Domains
	// Imported links are disabled until deliberately recreated after recovery.
	imported.GuestLinks = map[string]GuestLink{}
	imported.Backends = map[string]backendRecord{}
	for bid, b := range s.st.Backends {
		b.IsDefault = false
		imported.Backends[bid] = b
	}
	encrypted, e := s.encrypt(blob.Config{ID: id, Provider: "local", Root: root})
	if e != nil {
		return e
	}
	imported.Backends[id] = backendRecord{Backend: Backend{ID: id, Name: "Restored backup", Provider: "local", IsDefault: true, CreatedAt: now()}, Secret: encrypted}
	for aid, a := range imported.Artifacts {
		owner, ok := imported.Users[a.OwnerID]
		if !ok || owner.ID == "" {
			return errors.New("artifact owner missing")
		}
		a.OwnerName = owner.Name
		imported.Artifacts[aid] = a
	}
	if e = s.mutate(func() error {
		s.st = imported
		s.initFeatureState()
		s.audit(p, "backup.restored", id, "Objects restored to new local storage; current recovery configuration preserved")
		return nil
	}); e != nil {
		return e
	}
	s.stores[id] = st
	committed = true
	return nil
}

func validateBackup(zr *zip.Reader) (backupManifest, map[string]*zip.File, error) {
	var m backupManifest
	files := map[string]*zip.File{}
	if len(zr.File) > backupMaxObjects+1 {
		return m, nil, errors.New("too many entries")
	}
	var total uint64
	for _, f := range zr.File {
		if !safePath(f.Name) || f.FileInfo().IsDir() || !f.Mode().IsRegular() || f.Method != zip.Store || files[f.Name] != nil || f.UncompressedSize64 > uint64(backupMaxBytes) {
			return m, nil, errors.New("invalid archive entry")
		}
		total += f.UncompressedSize64
		if total > uint64(backupMaxBytes) {
			return m, nil, errors.New("archive too large")
		}
		files[f.Name] = f
	}
	mf := files["manifest.json"]
	if mf == nil || mf.UncompressedSize64 > uint64(backupStateMax*2) {
		return m, nil, errors.New("missing manifest")
	}
	rd, e := mf.Open()
	if e != nil {
		return m, nil, e
	}
	raw, e := io.ReadAll(io.LimitReader(rd, backupStateMax*2+1))
	rd.Close()
	if e != nil {
		return m, nil, e
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&m); e != nil {
		return m, nil, e
	}
	if m.Format != 1 || len(m.State) == 0 || len(m.State) > int(backupStateMax) || len(m.Objects) > backupMaxObjects || len(files) != len(m.Objects)+1 {
		return m, nil, errors.New("invalid manifest")
	}
	seen := map[string]bool{}
	identities := map[string]bool{}
	for _, o := range m.Objects {
		f := files[o.Entry]
		identity := o.BackendID + "\x00" + o.Key
		if !strings.HasPrefix(o.Entry, "objects/") || f == nil || seen[o.Entry] || identities[identity] || o.Size < 0 || uint64(o.Size) != f.UncompressedSize64 || len(o.SHA256) != 64 {
			return m, nil, errors.New("invalid object manifest")
		}
		if _, e = hex.DecodeString(o.SHA256); e != nil {
			return m, nil, e
		}
		seen[o.Entry] = true
		identities[identity] = true
	}
	return m, files, nil
}
func validateBackupState(st state, m backupManifest) error {
	objects := map[string]backupObject{}
	for _, o := range m.Objects {
		objects[o.BackendID+"\x00"+o.Key] = o
	}
	slugs := map[string]bool{}
	versions := map[string]bool{}
	if st.Artifacts == nil || st.Versions == nil || st.Users == nil {
		return errors.New("missing state")
	}
	if st.Policy.QuotaBytes < 1 || st.Policy.QuotaBytes > 1<<60 || st.Policy.MaxUploadBytes < 1 || st.Policy.MaxUploadBytes > 1<<40 || st.Policy.RetentionDays < 0 || st.Policy.RetentionDays > 36500 || (st.Policy.OverflowPolicy != "reject" && st.Policy.OverflowPolicy != "delete_oldest") {
		return errors.New("invalid backup policy")
	}
	emails := map[string]bool{}
	for uid, u := range st.Users {
		email, e := emailValue(u.Email)
		if e != nil || emails[email] {
			return errors.New("invalid or duplicate backup email")
		}
		emails[email] = true
		if uid == "" || uid != u.ID || (u.Role != "admin" && u.Role != "member") {
			return errors.New("invalid backup identity")
		}
	}
	for id, a := range st.Artifacts {
		if id == "" || a.ID != id || !slugOK(a.Slug) || slugs[a.Slug] || a.OwnerID == "" {
			return errors.New("invalid artifact")
		}
		slugs[a.Slug] = true
		if owner, ok := st.Users[a.OwnerID]; !ok || owner.ID != a.OwnerID {
			return errors.New("missing artifact owner")
		}
		if a.Visibility != "private" && a.Visibility != "organization" && a.Visibility != "selected" && a.Visibility != "public" {
			return errors.New("invalid artifact visibility")
		}
		var total int64
		for _, v := range st.Versions[id] {
			if v.ID == "" || v.ArtifactID != id || versions[v.ID] || len(v.Files) == 0 {
				return errors.New("invalid version")
			}
			versions[v.ID] = true
			var size int64
			paths := map[string]bool{}
			for _, f := range v.Files {
				o, ok := objects[v.BackendID+"\x00"+f.Key]
				if !ok || f.Size < 0 || f.Size != o.Size || !safePath(f.Path) || paths[f.Path] || strings.ContainsAny(f.MIMEType, "\r\n\x00") {
					return errors.New("invalid version file")
				}
				paths[f.Path] = true
				size += f.Size
			}
			if !paths["index.html"] {
				return errors.New("missing artifact index")
			}
			if size != v.SizeBytes {
				return errors.New("invalid version size")
			}
			total += size
		}
		if a.CurrentVersionID == "" {
			return errors.New("missing current version")
		}
		found := false
		official := a.PublishedVersionID == ""
		for _, v := range st.Versions[id] {
			if v.ID == a.PublishedVersionID && (!a.ReviewEnabled || v.Published) {
				official = true
			}
			if v.ID == a.CurrentVersionID {
				found = true
			}
		}
		if !found || !official || total < 0 {
			return errors.New("invalid current version")
		}
	}
	for aid := range st.Versions {
		if _, ok := st.Artifacts[aid]; !ok {
			return errors.New("orphan version")
		}
	}
	return nil
}
