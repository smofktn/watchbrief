package config

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)
func TestOriginBoundary(t *testing.T) {
 for _,tc := range []struct{origin,mode string; ok,secure bool}{
  {"http://127.0.0.1:5173","local-http",true,false},
  {"http://localhost:5173","local-http",true,false},
  {"https://watchbrief.example","https",true,true},
  {"http://watchbrief.example","local-http",false,false},
  {"http://localhost:5173","https",false,false},
  {"https://watchbrief.example/","https",false,false},
  {"https://user:password@watchbrief.example","https",false,false},
  {"https://watchbrief.example?x=y","https",false,false},
  {"https://watchbrief.example","",false,false},
 } {
  secure,err:=Origin(tc.origin,tc.mode)
  if (err==nil)!=tc.ok || secure!=tc.secure { t.Errorf("origin boundary failed for %s/%s",tc.origin,tc.mode) }
 }
}
func TestPrivateDBFile(t *testing.T) {
 file:=filepath.Join(t.TempDir(),"db.json")
 body:=[]byte(`{"host":"127.0.0.1","port":55432,"database":"watchbrief","user":"watchbrief_api","password":"fictional"}`)
 if err:=os.WriteFile(file,body,0600);err!=nil {t.Fatal(err)}
 if _,err:=ReadDB(file);err!=nil {t.Fatal(err)}
 if err:=os.Chmod(file,0644);err!=nil {t.Fatal(err)}
 if _,err:=ReadDB(file);err==nil {t.Fatal("world-readable secret accepted")}
}

func TestPrivateDBSizeAndTrailingData(t *testing.T) {
 file:=filepath.Join(t.TempDir(),"db.json")
 valid:=`{"host":"127.0.0.1","port":55432,"database":"watchbrief","user":"watchbrief_api","password":"fictional"}`
 for _,body:=range []string{valid+strings.Repeat(" ",8192), valid+` {}`, strings.Replace(valid,"127.0.0.1","example.invalid",1)} {
  if err:=os.WriteFile(file,[]byte(body),0600);err!=nil {t.Fatal(err)}
  if _,err:=ReadDB(file);err==nil {t.Fatal("invalid DB file accepted")}
 }
}
