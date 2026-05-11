package ssh

import (
	"fmt"
	"os"
	"strconv"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// MachineConfig holds the SSH connection parameters for a machine.
type MachineConfig struct {
	Name              string
	IP                string
	SSHPort           int32
	SSHUser           string
	SSHPrivateKeyPath string
}

// Client wraps an SSH connection and an SFTP client on top of it.
type Client struct {
	sshConn    *ssh.Client
	SFTPClient *sftp.Client
}

// Close shuts down the SFTP client and the underlying SSH connection.
func (c *Client) Close() {
	if c.SFTPClient != nil {
		c.SFTPClient.Close()
	}
	if c.sshConn != nil {
		c.sshConn.Close()
	}
}

// NewClient establishes an SSH+SFTP connection to the given machine.
func NewClient(cfg MachineConfig) (*Client, error) {
	sshConfig := ssh.ClientConfig{
		User: cfg.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
				keyBytes, err := os.ReadFile(cfg.SSHPrivateKeyPath)
				if err != nil {
					return nil, fmt.Errorf("read private key: %w", err)
				}
				key, err := ssh.ParsePrivateKey(keyBytes)
				if err != nil {
					return nil, fmt.Errorf("parse private key: %w", err)
				}
				return []ssh.Signer{key}, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
	}

	addr := cfg.IP + ":" + strconv.Itoa(int(cfg.SSHPort))
	sshConn, err := ssh.Dial("tcp", addr, &sshConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	sftpClient, err := sftp.NewClient(sshConn)
	if err != nil {
		sshConn.Close()
		return nil, fmt.Errorf("sftp client for %s: %w", addr, err)
	}

	return &Client{sshConn: sshConn, SFTPClient: sftpClient}, nil
}
