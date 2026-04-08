package github

import "testing"

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		owner   string
		repo    string
		wantErr bool
	}{
		{"https with .git", "https://github.com/owner/repo.git", "owner", "repo", false},
		{"https without .git", "https://github.com/owner/repo", "owner", "repo", false},
		{"ssh with .git", "git@github.com:owner/repo.git", "owner", "repo", false},
		{"ssh without .git", "git@github.com:owner/repo", "owner", "repo", false},
		{"non-github https", "https://gitlab.com/owner/repo.git", "", "", true},
		{"https missing repo", "https://github.com/owner", "", "", true},
		{"ssh missing repo", "git@github.com:owner", "", "", true},
		{"empty string", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseOwnerRepo(tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseOwnerRepo(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
			if err == nil && (owner != tt.owner || repo != tt.repo) {
				t.Errorf("got (%q, %q), want (%q, %q)", owner, repo, tt.owner, tt.repo)
			}
		})
	}
}

func TestForkURL(t *testing.T) {
	pr := PRInfo{
		ForkCloneURL: "https://github.com/alice/repo.git",
		ForkSSHURL:   "git@github.com:alice/repo.git",
	}
	tests := []struct {
		name      string
		originURL string
		want      string
	}{
		{
			"https origin → https fork",
			"https://github.com/owner/repo.git",
			"https://github.com/alice/repo.git",
		},
		{"ssh origin → ssh fork", "git@github.com:owner/repo.git", "git@github.com:alice/repo.git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pr.ForkURL(tt.originURL)
			if got != tt.want {
				t.Errorf("ForkURL(%q) = %q, want %q", tt.originURL, got, tt.want)
			}
		})
	}
}
