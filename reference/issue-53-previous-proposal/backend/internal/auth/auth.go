// Package auth is shared by the API and the administrative user creation command.
package auth

import (
 "crypto/rand"
 "crypto/sha256"
 "crypto/subtle"
 "encoding/base64"
 "encoding/hex"
 "errors"
 "regexp"
 "strings"
 "time"
 "unicode/utf8"

 "github.com/oklog/ulid/v2"
 "golang.org/x/crypto/argon2"
)

const passwordPrefix = "$argon2id$v=19$m=19456,t=2,p=1$"
var loginPattern = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
var ErrPasswordHash = errors.New("invalid password hash")

func LoginID(input string) (string, bool) {
 // Only ASCII uppercase is normalized; whitespace and non-ASCII stay invalid.
 input = strings.Map(func(r rune) rune { if r >= 'A' && r <= 'Z' { return r + ('a'-'A') }; return r }, input)
 return input, loginPattern.MatchString(input)
}

func ValidPassword(password string) bool {
 // No trimming, normalization, composition rule or silent truncation.
 n := utf8.RuneCountInString(password)
 return utf8.ValidString(password) && n >= 15 && n <= 128 && len(password) <= 512
}

func HashPassword(password string) (string, error) {
 if !ValidPassword(password) { return "", errors.New("password must contain 15 to 128 Unicode characters") }
 salt := make([]byte, 16)
 if _, err := rand.Read(salt); err != nil { return "", err }
 key := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
 return passwordPrefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

func VerifyPassword(password, encoded string) (bool, error) {
 // Accept only our bounded encoding. A corrupt DB value cannot request unbounded memory.
 if !strings.HasPrefix(encoded, passwordPrefix) { return false, ErrPasswordHash }
 parts := strings.Split(strings.TrimPrefix(encoded, passwordPrefix), "$")
 if len(parts) != 2 { return false, ErrPasswordHash }
 salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[0])
 if err != nil || len(salt) != 16 { return false, ErrPasswordHash }
 want, err := base64.RawStdEncoding.Strict().DecodeString(parts[1])
 if err != nil || len(want) != 32 { return false, ErrPasswordHash }
 got := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
 return subtle.ConstantTimeCompare(want, got) == 1, nil
}

func NewID() (string, error) {
 id, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
 return id.String(), err
}
func NewToken() (string, error) {
 raw := make([]byte, 32)
 if _, err := rand.Read(raw); err != nil { return "", err }
 return hex.EncodeToString(raw), nil
}
func TokenHash(token string) ([]byte, bool) {
 raw, err := hex.DecodeString(token)
 if err != nil || len(raw) != 32 || token != strings.ToLower(token) { return nil, false }
 digest := sha256.Sum256(raw)
 return digest[:], true
}
