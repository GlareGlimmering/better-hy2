package cmd

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"h-ui/dao"
	"h-ui/model/constant"
	"h-ui/util"
)

func resetTestDatabase(t *testing.T) string {
	t.Helper()
	dataRoot := t.TempDir()
	t.Setenv("HUI_DATA", dataRoot)
	return filepath.Join(dataRoot, filepath.FromSlash(constant.SqliteDBPath))
}

func openResetTestDatabase(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestResetAdminCredentialsInitializesEmptyDataDirectory(t *testing.T) {
	databasePath := resetTestDatabase(t)
	username, password, err := resetAdminCredentials()
	if err != nil {
		t.Fatalf("reset empty database: %v", err)
	}
	if username == "" || password == "" || username == "sysadmin" {
		t.Fatalf("unexpected generated credentials: username=%q password-empty=%v", username, password == "")
	}
	if _, err := os.Stat(databasePath); err != nil {
		t.Fatalf("SQLite database was not created: %v", err)
	}

	db := openResetTestDatabase(t, databasePath)
	for _, table := range []string{"account", "config", "panel_admin"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("table %s count=%d err=%v", table, count, err)
		}
	}
	var storedUsername, storedPassword string
	if err := db.QueryRow("SELECT username, password_hash FROM panel_admin").Scan(&storedUsername, &storedPassword); err != nil {
		t.Fatal(err)
	}
	if storedUsername != username || !util.VerifyPanelPassword(storedPassword, password) {
		t.Fatal("administrator was not reset correctly")
	}

	keys := []string{
		constant.HUIWebPort,
		constant.SubscriptionNodeNameTemplate,
		constant.SubscriptionServerTemplate,
		constant.XrayDraftConfig,
		constant.XrayActiveConfig,
	}
	var initialized int
	if err := db.QueryRow(`SELECT COUNT(*) FROM config WHERE key IN (?, ?, ?, ?, ?)`, keys[0], keys[1], keys[2], keys[3], keys[4]).Scan(&initialized); err != nil || initialized != len(keys) {
		t.Fatalf("default configuration count=%d want=%d err=%v", initialized, len(keys), err)
	}
}

func TestResetAdminCredentialsPreservesExistingUsersAndConfig(t *testing.T) {
	databasePath := resetTestDatabase(t)
	if _, _, err := resetAdminCredentials(); err != nil {
		t.Fatal(err)
	}
	db := openResetTestDatabase(t, databasePath)
	if _, err := db.Exec(`INSERT INTO account (username, pass, con_pass, quota, role) VALUES (?, ?, ?, ?, ?)`, "kept-user", "kept-pass", "kept.connection", 12345, "user"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE config SET value=? WHERE key=?`, "/kept-context", constant.HUIWebContext); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, _, err := resetAdminCredentials(); err != nil {
		t.Fatalf("reset existing database: %v", err)
	}
	db = openResetTestDatabase(t, databasePath)
	var pass, connection string
	var quota int64
	if err := db.QueryRow(`SELECT pass, con_pass, quota FROM account WHERE username='kept-user'`).Scan(&pass, &connection, &quota); err != nil {
		t.Fatal(err)
	}
	if pass != "kept-pass" || connection != "kept.connection" || quota != 12345 {
		t.Fatalf("existing user changed: pass=%q connection=%q quota=%d", pass, connection, quota)
	}
	var context string
	if err := db.QueryRow(`SELECT value FROM config WHERE key=?`, constant.HUIWebContext).Scan(&context); err != nil || context != "/kept-context" {
		t.Fatalf("existing config changed: value=%q err=%v", context, err)
	}
}

func TestSqliteSchemaInitializationIsIdempotent(t *testing.T) {
	databasePath := resetTestDatabase(t)
	for attempt := 0; attempt < 2; attempt++ {
		if err := dao.InitSqliteSchema(); err != nil {
			t.Fatalf("schema initialization %d: %v", attempt+1, err)
		}
		if err := dao.CloseSqliteDB(); err != nil {
			t.Fatalf("close database %d: %v", attempt+1, err)
		}
	}
	db := openResetTestDatabase(t, databasePath)
	var admins int
	if err := db.QueryRow(`SELECT COUNT(*) FROM panel_admin`).Scan(&admins); err != nil || admins != 0 {
		t.Fatalf("default administrator count=%d err=%v", admins, err)
	}
	var total, distinct int
	if err := db.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT key) FROM config`).Scan(&total, &distinct); err != nil || total != distinct {
		t.Fatalf("duplicate config rows: total=%d distinct=%d err=%v", total, distinct, err)
	}
}

func TestResetUsesExecutableDirectoryOutsideCurrentWorkingDirectory(t *testing.T) {
	installDir := t.TempDir()
	otherDir := t.TempDir()
	originalExecutablePath := executablePath
	executablePath = func() (string, error) { return filepath.Join(installDir, "h-ui"), nil }
	t.Cleanup(func() { executablePath = originalExecutablePath })
	t.Setenv("HUI_DATA", "")
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(otherDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWorkingDirectory) })

	username, _, err := resetAdminCredentials()
	if err != nil {
		t.Fatalf("reset outside executable directory: %v", err)
	}
	expectedDatabase := filepath.Join(installDir, filepath.FromSlash(constant.SqliteDBPath))
	db := openResetTestDatabase(t, expectedDatabase)
	var storedUsername string
	if err = db.QueryRow(`SELECT username FROM panel_admin`).Scan(&storedUsername); err != nil || storedUsername != username {
		t.Fatalf("administrator was not updated in standard database: username=%q err=%v", storedUsername, err)
	}
	unexpectedDatabase := filepath.Join(otherDir, filepath.FromSlash(constant.SqliteDBPath))
	if _, err = os.Stat(unexpectedDatabase); !os.IsNotExist(err) {
		t.Fatalf("reset created a database relative to cwd: %s (%v)", unexpectedDatabase, err)
	}
}

func TestCustomResetSubcommandsPreserveUnchangedCredentials(t *testing.T) {
	path := resetTestDatabase(t)
	first, second := "first-password-1234", "second-password-5678"
	run := func(input string, args ...string) (string, error) {
		var output bytes.Buffer
		command := newResetCommand()
		command.SetArgs(args)
		command.SetIn(strings.NewReader(input))
		command.SetOut(&output)
		command.SetErr(&output)
		err := command.Execute()
		if strings.Contains(output.String(), first) || strings.Contains(output.String(), second) {
			t.Fatal("custom password was printed")
		}
		return output.String(), err
	}
	if _, err := run(first+"\n", "--username", "first-admin", "--password-stdin"); err != nil {
		t.Fatal(err)
	}
	db := openResetTestDatabase(t, path)
	read := func() (string, string) {
		t.Helper()
		var username, hash string
		if err := db.QueryRow("SELECT username, password_hash FROM panel_admin").Scan(&username, &hash); err != nil {
			t.Fatal(err)
		}
		return username, hash
	}
	name, hash := read()
	if name != "first-admin" || !util.VerifyPanelPassword(hash, first) {
		t.Fatal("custom credentials were not stored")
	}
	if _, err := run("", "username", "next-admin"); err != nil {
		t.Fatal(err)
	}
	if newName, newHash := read(); newName != "next-admin" || newHash != hash {
		t.Fatal("rename did not preserve the password")
	}
	if _, err := run(second+"\n", "password", "--stdin"); err != nil {
		t.Fatal(err)
	}
	if name, hash := read(); name != "next-admin" || !util.VerifyPanelPassword(hash, second) {
		t.Fatal("password reset did not preserve the username")
	}
	if _, err := run("short\n", "password", "--stdin"); err == nil {
		t.Fatal("accepted invalid password")
	}
	if name, hash := read(); name != "next-admin" || !util.VerifyPanelPassword(hash, second) {
		t.Fatal("invalid reset changed the administrator")
	}
}
