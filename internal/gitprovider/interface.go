package gitprovider

type PRContext struct {
	PRNumber   int
	Repository string // e.g. "owner/repo"
	HeadBranch string
	BaseBranch string
	Title      string
	Body       string
	Commits    []string
	Diff       string
}

// GitProvider defines methods a Git host must implement.
type GitProvider interface {
	GetPRContext() (*PRContext, error)
	PostComment(comment string) error
}
