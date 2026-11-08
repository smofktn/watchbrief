// Package config loads private connection configuration without logging its contents.
package config

import (
 "bytes"
 "encoding/json"
 "errors"
 "io"
 "net/url"
 "os"
 "strings"
)

type DB struct {
 Host string `json:"host"`
 Port uint16 `json:"port"`
 Database string `json:"database"`
 User string `json:"user"`
 Password string `json:"password"`
}

// ReadDB requires a regular owner-only file. Use separate files for API and admin.
func ReadDB(path string) (DB, error) {
 var cfg DB
 f, err := os.Open(path)
 if err != nil { return cfg, errors.New("cannot open DB configuration") }
 defer f.Close()
 info, err := f.Stat()
 if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() & 0077 != 0 { return cfg, errors.New("DB configuration must be a regular owner-only file (0600)") }
 body, err := io.ReadAll(io.LimitReader(f, 8193))
 if err != nil || len(body) > 8192 { return DB{}, errors.New("invalid DB configuration size") }
 dec := json.NewDecoder(bytes.NewReader(body))
 dec.DisallowUnknownFields()
 if err := dec.Decode(&cfg); err != nil { return DB{}, errors.New("invalid DB configuration") }
 var extra any
 if dec.Decode(&extra) != io.EOF { return DB{}, errors.New("invalid DB configuration") }
 if cfg.Host != "127.0.0.1" || cfg.Port == 0 || cfg.Database == "" || cfg.User == "" || cfg.Password == "" { return DB{}, errors.New("DB configuration requires localhost, port, database, user and password") }
 return cfg, nil
}

func Origin(raw, mode string) (bool, error) {
 u, err := url.Parse(raw)
 if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || strings.ContainsAny(raw, "\r\n") { return false, errors.New("APP_ORIGIN must be an exact origin without path") }
 switch mode {
 case "https":
  if u.Scheme != "https" { return false, errors.New("https mode requires an https APP_ORIGIN") }
  return true, nil
 case "local-http":
  if u.Scheme != "http" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1") { return false, errors.New("local-http requires a localhost http origin") }
  return false, nil
 default:
  return false, errors.New("COOKIE_MODE must be https or local-http")
 }
}
