package config

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// SSHConfig holds the resolved SSH configuration paths.
type SSHConfig struct {
	PrivateKeysPath string
	KnownHostsPath  string
}

// ValidateSSHConfig checks at startup that:
//   - ssh.known_hosts_path exists on disk (fatal if not)
//   - every private key file found under ssh.private_keys_path has strict
//     0400 permissions; if not, a WARNING is logged but startup continues.
func ValidateSSHConfig(cfg SSHConfig, logger *slog.Logger) error {
	// ── known_hosts ──────────────────────────────────────────────────────────
	if _, err := os.Stat(cfg.KnownHostsPath); os.IsNotExist(err) {
		return fmt.Errorf("ssh.known_hosts_path %q does not exist", cfg.KnownHostsPath)
	} else if err != nil {
		return fmt.Errorf("ssh.known_hosts_path %q: %w", cfg.KnownHostsPath, err)
	}

	// ── private keys ─────────────────────────────────────────────────────────
	if err := checkKeyPermissions(cfg.PrivateKeysPath, logger); err != nil {
		return err
	}

	return nil
}

// checkKeyPermissions scans the directory at dirPath and logs a warning for
// every file whose permissions are not strictly 0400.
func checkKeyPermissions(dirPath string, logger *slog.Logger) error {
	info, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Errorf("ssh.private_keys_path %q: %w", dirPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("ssh.private_keys_path %q is not a directory", dirPath)
	}

	return filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			logger.Warn("ssh: could not stat key file", "path", path, "error", err)
			return nil
		}

		// Strict check: only owner-read (0400) is acceptable.
		if fi.Mode().Perm() != 0400 {
			logger.Warn(
				"ssh: private key has insecure permissions (expected 0400)",
				"path", path,
				"permissions", fi.Mode().Perm().String(),
			)
		}

		return nil
	})
}
