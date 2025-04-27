package change

import "testing"

func TestParseGithubURL(t *testing.T) {
	urls := []struct {
		url   string
		owner string
		repo  string
	}{
		{"https://github.com/yola1107/kratos.git", "yola1107", "kratos"},
		{"https://github.com/yola1107/kratos", "yola1107", "kratos"},
		{"git@github.com:yola1107/kratos.git", "yola1107", "kratos"},
		{"https://github.com/yola1107/yola1107.dev.git", "yola1107", "yola1107.dev"},
	}
	for _, url := range urls {
		owner, repo := ParseGithubURL(url.url)
		if owner != url.owner {
			t.Fatalf("owner want: %s, got: %s", owner, url.owner)
		}
		if repo != url.repo {
			t.Fatalf("repo want: %s, got: %s", repo, url.repo)
		}
	}
}
