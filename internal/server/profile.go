package server

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"mime"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"
)

const photoUploadMax = 5 << 20
const photoInputDimensionMax = 2048
const photoDimensionMax = 256
const photoStoredMax = 300 << 10
const photoStateMax = 8 << 20

// Photo bytes are private state, never embedded in User/Organization API JSON.
// Keeping them in the SQLite snapshot makes commit/rollback and backup atomic.
type profilePhoto struct {
	Data []byte
	MIME string
}

func photoURL(path string, photo *profilePhoto) string {
	if photo == nil {
		return ""
	}
	return path + "?v=" + digest(string(photo.Data))[:16]
}

func sanitizePhoto(data []byte) (*profilePhoto, error) {
	invalid := func() (*profilePhoto, error) {
		return nil, fail(400, "invalid_photo", "Choose a valid JPEG or PNG image up to 2048 by 2048 pixels")
	}
	if len(data) == 0 || len(data) > photoUploadMax {
		return invalid()
	}
	sniffed := http.DetectContentType(data)
	if sniffed != "image/jpeg" && sniffed != "image/png" {
		return invalid()
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || (format != "jpeg" && format != "png") || "image/"+format != sniffed || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > photoInputDimensionMax || cfg.Height > photoInputDimensionMax {
		return invalid()
	}
	decoded, decodedFormat, e := image.Decode(bytes.NewReader(data))
	if e != nil || decodedFormat != format || decoded.Bounds().Dx() != cfg.Width || decoded.Bounds().Dy() != cfg.Height {
		return invalid()
	}
	var img image.Image = decoded
	if cfg.Width > photoDimensionMax || cfg.Height > photoDimensionMax {
		img = resizePhoto(decoded)
	}
	var out bytes.Buffer
	if format == "jpeg" {
		e = jpeg.Encode(&out, img, &jpeg.Options{Quality: 88})
	} else {
		e = png.Encode(&out, img)
	}
	if e != nil || out.Len() > photoStoredMax {
		return invalid()
	}
	return &profilePhoto{Data: out.Bytes(), MIME: sniffed}, nil
}

// Bilinear interpolation in premultiplied color space preserves alpha edges.
func resizePhoto(src image.Image) image.Image {
	b := src.Bounds()
	scale := math.Min(float64(photoDimensionMax)/float64(b.Dx()), float64(photoDimensionMax)/float64(b.Dy()))
	w, h := max(1, int(math.Round(float64(b.Dx())*scale))), max(1, int(math.Round(float64(b.Dy())*scale)))
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		sy := math.Max(0, (float64(y)+0.5)*float64(b.Dy())/float64(h)-0.5)
		y0 := int(sy)
		dy := sy - float64(y0)
		for x := 0; x < w; x++ {
			sx := math.Max(0, (float64(x)+0.5)*float64(b.Dx())/float64(w)-0.5)
			x0 := int(sx)
			dx := sx - float64(x0)
			var rgba [4]float64
			for iy := 0; iy < 2; iy++ {
				for ix := 0; ix < 2; ix++ {
					px := b.Min.X + min(x0+ix, b.Dx()-1)
					py := b.Min.Y + min(y0+iy, b.Dy()-1)
					r, g, bl, a := src.At(px, py).RGBA()
					wx, wy := 1-dx, 1-dy
					if ix == 1 {
						wx = dx
					}
					if iy == 1 {
						wy = dy
					}
					for i, v := range []uint32{r, g, bl, a} {
						rgba[i] += float64(v) * wx * wy
					}
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(math.Round(rgba[0] / 257)), uint8(math.Round(rgba[1] / 257)), uint8(math.Round(rgba[2] / 257)), uint8(math.Round(rgba[3] / 257))})
		}
	}
	return dst
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request, p principal) error {
	if e := s.require(r, p, false, ""); e != nil {
		return e
	}
	if p.kind != "session" {
		return fail(403, "forbidden", "Profile changes require a session")
	}
	var in struct{ Name string }
	if e := decode(r, &in); e != nil {
		return e
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 100 || strings.ContainsFunc(name, unicode.IsControl) {
		return fail(400, "invalid_profile_name", "Name must contain between 1 and 100 characters without control characters")
	}
	if e := s.mutate(func() error {
		u := s.st.Users[p.user.ID]
		u.Name = name
		s.st.Users[u.ID] = u
		s.audit(p, "profile.updated", u.ID, "")
		return nil
	}); e != nil {
		return e
	}
	writeJSON(w, 200, map[string]any{"user": s.st.Users[p.user.ID].User})
	return nil
}

func (s *Server) profilePhotos(w http.ResponseWriter, r *http.Request, p principal) error {
	if e := s.require(r, p, false, "read"); e != nil {
		return e
	}
	organization := r.URL.Path == "/api/organization/photo"
	self := r.URL.Path == "/api/me/photo"
	id := p.user.ID
	if !organization && !self {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/members/"), "/")
		if len(parts) != 2 || parts[1] != "photo" || parts[0] == "" {
			return fail(404, "not_found", "Not found")
		}
		id = parts[0]
	}
	reading := r.Method == "GET" || r.Method == "HEAD"
	if !reading {
		if p.kind != "session" || (!organization && !self) || (organization && p.user.Role != "admin") {
			return fail(403, "forbidden", "An administrator can change the organization photo; members can change their own photo")
		}
		if r.Method != "PUT" && r.Method != "DELETE" {
			return fail(405, "method_not_allowed", "Method not allowed")
		}
	}
	u, exists := s.st.Users[id]
	if !organization && (!exists || u.Disabled) {
		return fail(404, "not_found", "Photo not found")
	}
	photo := s.st.OrganizationPhoto
	if !organization {
		photo = u.Photo
	}
	if reading {
		if photo == nil {
			return fail(404, "not_found", "Photo not found")
		}
		w.Header().Set("Content-Type", photo.MIME)
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		if r.Method == "GET" {
			_, e := w.Write(photo.Data)
			return e
		}
		w.WriteHeader(http.StatusOK)
		return nil
	}
	var replacement *profilePhoto
	if r.Method == "PUT" {
		defer r.Body.Close()
		contentType, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if e != nil || (contentType != "image/jpeg" && contentType != "image/png") {
			return fail(415, "invalid_photo", "JPEG or PNG content type required")
		}
		data, e := io.ReadAll(http.MaxBytesReader(w, r.Body, photoUploadMax))
		if e != nil {
			var limit *http.MaxBytesError
			if errors.As(e, &limit) {
				return fail(413, "photo_too_large", "Photo must be 5 MiB or smaller")
			}
			return fail(400, "invalid_photo", "Photo could not be read")
		}
		if contentType != http.DetectContentType(data) {
			return fail(415, "invalid_photo", "Photo content type does not match")
		}
		replacement, e = sanitizePhoto(data)
		if e != nil {
			return e
		}
		var total int
		for uid, member := range s.st.Users {
			if member.Photo != nil && (organization || uid != id) {
				total += len(member.Photo.Data)
			}
		}
		if !organization && s.st.OrganizationPhoto != nil {
			total += len(s.st.OrganizationPhoto.Data)
		}
		if total+len(replacement.Data) > photoStateMax {
			return fail(413, "photo_storage_full", "Photo storage is full; remove an unused photo first")
		}
	}
	if e := s.mutate(func() error {
		target, action := id, "profile.photo_changed"
		if organization {
			target, action = s.st.Organization.ID, "organization.photo_changed"
			s.st.OrganizationPhoto = replacement
			s.st.Organization.AvatarURL = photoURL("/api/organization/photo", replacement)
		} else {
			u.Photo = replacement
			u.AvatarURL = photoURL("/api/members/"+id+"/photo", replacement)
			s.st.Users[id] = u
		}
		if replacement == nil {
			action = strings.TrimSuffix(action, "changed") + "removed"
		}
		s.audit(p, action, target, "")
		return nil
	}); e != nil {
		return e
	}
	if organization {
		writeJSON(w, 200, map[string]any{"organization": s.st.Organization})
	} else {
		writeJSON(w, 200, map[string]any{"user": s.st.Users[id].User})
	}
	return nil
}

// Validate and strip untrusted backup-supplied URLs and image metadata before
// recovery. Current identities (and their photos) still win in identity merging.
func normalizeBackupPhotos(st *state) error {
	total := 0
	clean := func(photo **profilePhoto) error {
		if *photo == nil {
			return nil
		}
		if len((*photo).Data) > photoStoredMax {
			return errors.New("invalid backup photo size")
		}
		p, e := sanitizePhoto((*photo).Data)
		if e != nil {
			return errors.New("invalid backup photo")
		}
		*photo = p
		total += len(p.Data)
		if total > photoStateMax {
			return errors.New("backup photos exceed storage limit")
		}
		return nil
	}
	if e := clean(&st.OrganizationPhoto); e != nil {
		return e
	}
	st.Organization.AvatarURL = photoURL("/api/organization/photo", st.OrganizationPhoto)
	for id, u := range st.Users {
		if e := clean(&u.Photo); e != nil {
			return e
		}
		u.AvatarURL = photoURL("/api/members/"+id+"/photo", u.Photo)
		st.Users[id] = u
	}
	return nil
}

// Identity merging can combine destination photos with imported-only users.
func validatePhotoStateSize(st state) error {
	total := 0
	if st.OrganizationPhoto != nil {
		total += len(st.OrganizationPhoto.Data)
	}
	for _, u := range st.Users {
		if u.Photo != nil {
			total += len(u.Photo.Data)
		}
	}
	if total > photoStateMax {
		return errors.New("merged profile photos exceed storage limit")
	}
	return nil
}
