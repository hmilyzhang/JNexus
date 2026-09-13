// JNexus Ops Platform — By JJ Zhang, Version 1.0

package service

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"

	"jnexus/internal/config"
)

var httpTimeoutClient = &http.Client{Timeout: 10 * time.Second}

// LocalUploadPath returns the server-side staging file path
func LocalUploadPath(name string) string {
	// keep only the file name from name to prevent path traversal
	return filepath.Join(config.Cfg.Storage.UploadDir, filepath.Base(name))
}

// uploadViaSSH uploads a file over an established SSH connection via SFTP
func uploadViaSSH(cli *gossh.Client, localPath, remoteDir, remoteName string) error {
	scli, err := sftp.NewClient(cli)
	if err != nil {
		return err
	}
	defer scli.Close()
	if err := mkdirAll(scli, remoteDir); err != nil {
		return err
	}
	local, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer local.Close()
	remote, err := scli.Create(remoteDir + "/" + remoteName)
	if err != nil {
		return err
	}
	defer remote.Close()
	if _, err := remote.ReadFrom(local); err != nil {
		return err
	}
	return nil
}

// FormatOutput truncates overly long output
func FormatOutput(s string, max int) string {
	if len(s) > max {
		return s[:max] + fmt.Sprintf("\n...(输出过长，已截断，共 %d 字节)", len(s))
	}
	return s
}
