// This package is responsible for committing and pushing the code to a git repo
// It is currently implemented by [gitcli.go]
package git

import "time"

type CommitFile struct {
	FolderName string
	FileName   string
	Code       string
}

type GitClient interface {
	Commit(files []CommitFile, commitMessage string, timestamp time.Time) error
	Push() error
}
