// Package i18n resolves which language a request wants (Indonesian or
// English) and translates message keys into that language. It intentionally
// stays lightweight — a map lookup plus fmt.Sprintf — rather than pulling in
// a full i18n library, since the API only needs flat, non-pluralized strings
// for response messages and validation errors.
package i18n

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
)

type Lang string

const (
	EN Lang = "en"
	ID Lang = "id"

	// DefaultLang is used when the request doesn't specify a language via
	// ?lang= or Accept-Language, and is also the fallback for any key that
	// has no translation for the resolved language.
	DefaultLang = EN

	contextKey = "lang"
)

// messages maps a message key to its text per language. Every key must have
// an EN entry — it's the fallback when a translation is missing or the
// resolved language isn't recognized.
var messages = map[string]map[Lang]string{
	// Validation (pkg/response.ValidationFailed)
	"validation.failed":       {EN: "validation failed", ID: "validasi gagal"},
	"validation.invalid_body": {EN: "invalid request body", ID: "isi permintaan tidak valid"},
	"validation.required":     {EN: "this field is required", ID: "wajib diisi"},
	"validation.email":        {EN: "must be a valid email address", ID: "harus berupa alamat email yang valid"},
	"validation.min":          {EN: "must be at least %s characters", ID: "minimal %s karakter"},
	"validation.max":          {EN: "must be at most %s characters", ID: "maksimal %s karakter"},
	"validation.len":          {EN: "must be exactly %s characters", ID: "harus tepat %s karakter"},
	"validation.numeric":      {EN: "must contain only numbers", ID: "hanya boleh berisi angka"},
	"validation.alphanum":     {EN: "must contain only letters and numbers", ID: "hanya boleh berisi huruf dan angka"},
	"validation.url":          {EN: "must be a valid URL", ID: "harus berupa URL yang valid"},
	"validation.gte":          {EN: "must be greater than or equal to %s", ID: "harus lebih besar atau sama dengan %s"},
	"validation.lte":          {EN: "must be less than or equal to %s", ID: "harus lebih kecil atau sama dengan %s"},
	"validation.oneof":        {EN: "must be one of: %s", ID: "harus salah satu dari: %s"},
	"validation.generic":      {EN: "failed validation on '%s'", ID: "gagal validasi pada '%s'"},
	"validation.invalid_date": {EN: "invalid date — use YYYY-MM-DD", ID: "tanggal tidak valid — gunakan format YYYY-MM-DD"},

	// Auth handler (internal/handler/auth.go)
	"auth.register.success":       {EN: "registration successful", ID: "registrasi berhasil"},
	"auth.register.failed":        {EN: "failed to register", ID: "gagal melakukan registrasi"},
	"auth.login.success":          {EN: "login successful", ID: "login berhasil"},
	"auth.login.failed":           {EN: "failed to login", ID: "gagal login"},
	"auth.profile.retrieved":      {EN: "profile retrieved", ID: "profil berhasil diambil"},
	"auth.profile.get_failed":     {EN: "failed to get profile", ID: "gagal mengambil profil"},
	"auth.profile.updated":        {EN: "profile updated", ID: "profil berhasil diperbarui"},
	"auth.profile.update_failed":  {EN: "failed to update profile", ID: "gagal memperbarui profil"},
	"auth.picture.uploaded":       {EN: "profile picture uploaded", ID: "foto profil berhasil diunggah"},
	"auth.picture.upload_failed":  {EN: "failed to upload profile picture", ID: "gagal mengunggah foto profil"},
	"auth.picture.required":       {EN: "file is required", ID: "berkas wajib diunggah"},
	"auth.picture.too_large":      {EN: "file must be at most 5 MB", ID: "berkas maksimal berukuran 5 MB"},
	"auth.picture.read_failed":    {EN: "failed to read file", ID: "gagal membaca berkas"},
	"auth.picture.invalid_type":   {EN: "only jpg, png, and webp images are allowed", ID: "hanya gambar jpg, png, dan webp yang diperbolehkan"},
	"auth.picture.process_failed": {EN: "failed to process file", ID: "gagal memproses berkas"},
	"auth.password.changed":       {EN: "password changed successfully", ID: "kata sandi berhasil diubah"},
	"auth.password.change_failed": {EN: "failed to change password", ID: "gagal mengubah kata sandi"},
	"auth.logout_all.success":     {EN: "logged out of all devices", ID: "berhasil keluar dari semua perangkat"},
	"auth.logout_all.failed":      {EN: "failed to log out of all devices", ID: "gagal keluar dari semua perangkat"},

	// JWT middleware (internal/middleware/jwt.go)
	"auth.token_required":  {EN: "authorization header with Bearer token required", ID: "header otorisasi dengan token Bearer wajib disertakan"},
	"auth.token_invalid":   {EN: "invalid or expired token", ID: "token tidak valid atau sudah kedaluwarsa"},
	"auth.session_revoked": {EN: "session has been revoked, please log in again", ID: "sesi telah dicabut, silakan masuk kembali"},

	// Sentinel errors surfaced to clients (internal/service/auth.go, internal/service/mention.go)
	"error.email_taken":            {EN: "email already registered", ID: "email sudah terdaftar"},
	"error.invalid_creds":          {EN: "invalid email or password", ID: "email atau kata sandi tidak valid"},
	"error.user_not_found":         {EN: "user not found", ID: "pengguna tidak ditemukan"},
	"error.storage_not_configured": {EN: "file storage is not configured", ID: "penyimpanan berkas belum dikonfigurasi"},
	"error.post_not_found":         {EN: "post not found", ID: "postingan tidak ditemukan"},
	"error.incorrect_password":     {EN: "current password is incorrect", ID: "kata sandi saat ini salah"},

	// Mentions handler (internal/handler/mention.go)
	"mentions.posts.list_failed":   {EN: "failed to list posts", ID: "gagal mengambil daftar postingan"},
	"mentions.posts.get_failed":    {EN: "failed to get post", ID: "gagal mengambil postingan"},
	"mentions.posts.invalid_id":    {EN: "invalid post id", ID: "id postingan tidak valid"},
	"mentions.posts.list_ok":       {EN: "posts retrieved", ID: "daftar postingan berhasil diambil"},
	"mentions.posts.get_ok":        {EN: "post retrieved", ID: "postingan berhasil diambil"},
	"mentions.comments.get_failed": {EN: "failed to get comments", ID: "gagal mengambil komentar"},
	"mentions.comments.get_ok":     {EN: "comments retrieved", ID: "daftar komentar berhasil diambil"},

	// Ingest handler (internal/handler/ingest.go)
	"ingest.posts.create_ok":     {EN: "post ingested", ID: "postingan berhasil disimpan"},
	"ingest.posts.create_failed": {EN: "failed to ingest post", ID: "gagal menyimpan postingan"},

	// Dashboard handler (internal/handler/dashboard.go)
	"dashboard.overview.get_ok":     {EN: "dashboard overview retrieved", ID: "ringkasan dashboard berhasil diambil"},
	"dashboard.overview.get_failed": {EN: "failed to get dashboard overview", ID: "gagal mengambil ringkasan dashboard"},

	// Sentiment handler (internal/handler/sentiment.go)
	"sentiment.overview.get_ok":     {EN: "sentiment overview retrieved", ID: "ringkasan sentimen berhasil diambil"},
	"sentiment.overview.get_failed": {EN: "failed to get sentiment overview", ID: "gagal mengambil ringkasan sentimen"},

	// Keyword handler (internal/handler/keywords.go)
	"keywords.list_ok":     {EN: "keywords retrieved", ID: "daftar keyword berhasil diambil"},
	"keywords.list_failed": {EN: "failed to list keywords", ID: "gagal mengambil daftar keyword"},
}

// T translates key into the given language, formatting it with args via
// fmt.Sprintf if any are provided. Falls back to EN, then to the raw key,
// if a translation is missing — so a missing entry degrades to visible
// English text instead of a blank message or a panic.
func T(lang Lang, key string, args ...any) string {
	entry, ok := messages[key]
	if !ok {
		return key
	}

	text, ok := entry[lang]
	if !ok {
		text, ok = entry[EN]
		if !ok {
			return key
		}
	}

	if len(args) > 0 {
		text = fmt.Sprintf(text, args...)
	}

	return capitalize(text)
}

// capitalize uppercases the first character so every response message reads
// as a proper sentence ("Email already registered", not "email already
// registered"), regardless of how it's written in the messages map below.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// FromContext reads the language resolved by Middleware for this request.
func FromContext(c *gin.Context) Lang {
	if value, ok := c.Get(contextKey); ok {
		if lang, ok := value.(Lang); ok {
			return lang
		}
	}
	return DefaultLang
}

// Middleware resolves the request's language from an explicit ?lang= query
// param (handy for testing via Swagger/browser) or, failing that, the
// Accept-Language header, and stores it on the context for handlers and
// pkg/response to read via FromContext.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := DefaultLang
		if q := c.Query("lang"); q != "" {
			lang = parse(q)
		} else if header := c.GetHeader("Accept-Language"); header != "" {
			lang = parse(header)
		}
		c.Set(contextKey, lang)
		c.Next()
	}
}

func parse(value string) Lang {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "id") {
		return ID
	}
	return EN
}
