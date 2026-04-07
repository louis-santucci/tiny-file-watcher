package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"tiny-file-watcher/server/config"
	"tiny-file-watcher/server/database"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// dialSFTP establishes an SSH connection to the given machine and returns an
// SFTP-backed RemoteFS. The caller is responsible for closing the underlying
// connection when done (currently handled by the SFTP client's lifecycle).
func dialSFTP(logger *slog.Logger, machine *database.Machine, sshConfig *config.SSHConfig) (RemoteFS, error) {
	logger.Debug("private key path", "path", filepath.Join(sshConfig.PrivateKeysPath, machine.SSHKeyName))

	clientConfig := ssh.ClientConfig{
		User: machine.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
				keyPath := filepath.Join(sshConfig.PrivateKeysPath, machine.SSHKeyName)
				keyBytes, err := os.ReadFile(keyPath)
				if err != nil {
					return nil, err
				}
				key, err := ssh.ParsePrivateKey(keyBytes)
				if err != nil {
					return nil, err
				}
				return []ssh.Signer{key}, nil
			}),
		},
		// TODO: replace with a proper host-key verification strategy.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := machine.IP + ":" + strconv.Itoa(int(machine.SSHPort))
	logger.Debug("sync: dialling SSH", "addr", addr)

	conn, err := ssh.Dial("tcp", addr, &clientConfig)
	if err != nil {
		logger.Error("failed to connect to machine", "error", err)
		return nil, err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		logger.Error("failed to create SFTP client", "error", err)
		if cerr := conn.Close(); cerr != nil {
			logger.Warn("sync: failed to close SSH connection after SFTP error", "error", cerr)
		}
		return nil, err
	}

	return sftpRemoteFS{c: client}, nil
}
