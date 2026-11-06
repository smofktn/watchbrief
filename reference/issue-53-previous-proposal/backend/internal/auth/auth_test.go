package auth

import (
 "bytes"
 "strings"
 "testing"
)

func TestPasswordAndTokens(t *testing.T) {
 password := "架空ユーザー用の十分に長いパスワードです"
 first, err := HashPassword(password)
 if err != nil { t.Fatal(err) }
 second, err := HashPassword(password)
 if err != nil || first == second { t.Fatal("hashes need independent salts") }
 ok, err := VerifyPassword(password, first)
 if err != nil || !ok { t.Fatal("password did not verify") }
 if ok, err := VerifyPassword(password+"!", first); err != nil || ok { t.Fatal("wrong password accepted") }
 for _, corrupt := range []string{"", "$argon2id$v=19$m=999999999,t=2,p=1$x$y", first+"$extra"} {
  if _, err := VerifyPassword(password, corrupt); err == nil { t.Fatal("corrupt hash accepted") }
 }
 if ValidPassword(strings.Repeat("a", 14)) || ValidPassword(strings.Repeat("a",129)) || ValidPassword(string([]byte{255})) { t.Fatal("invalid password accepted") }
 token, err := NewToken()
 if err != nil || len(token) != 64 { t.Fatal("invalid token") }
 digest, valid := TokenHash(token)
 if !valid || len(digest) != 32 || bytes.Contains(digest, []byte(token)) { t.Fatal("invalid token hash") }
 for _, invalid := range []string{"", "a", strings.Repeat("g",64), strings.Repeat("A",64)} {
  if _, valid := TokenHash(invalid); valid { t.Fatal("invalid token accepted") }
 }
}

func TestLoginID(t *testing.T) {
 normalized, valid := LoginID("Example_USER-1")
 if !valid || normalized != "example_user-1" { t.Fatal("normalization failed") }
 for _, invalid := range []string{"", " user", "user ", "Ａ", "foo@example", strings.Repeat("a",65), "a\n"} {
  if _, valid := LoginID(invalid); valid { t.Fatal("invalid login ID accepted") }
 }
}
