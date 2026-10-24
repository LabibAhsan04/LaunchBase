package github

import "testing"

func TestParseRepoURL(t *testing.T) {
	ok := []string{
		"https://github.com/acme/web",
		"https://github.com/acme/web.git",
		"https://github.com/acme/web/",
		"https://GitHub.com/Acme/Web.git",
	}
	for _, raw := range ok {
		repo, err := ParseRepoURL(raw)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if repo.CloneURL != "https://github.com/"+repo.Owner+"/"+repo.Name+".git" {
			t.Fatalf("%s clone url %s", raw, repo.CloneURL)
		}
	}

	bad := []string{
		"",
		"   ",
		"git@github.com:acme/web.git",
		"ssh://git@github.com/acme/web.git",
		"file:///tmp/web",
		"/tmp/web",
		"./web",
		"http://github.com/acme/web",
		"https://gitlab.com/acme/web",
		"https://github.com/acme",
		"https://github.com/acme/web/tree/main",
		"https://user:token@github.com/acme/web",
		"https://github.com/acme/web?ref=main",
		"https://github.com/acme/web#readme",
		"https://github.com/acme/web\n",
		"https://github.com/../web",
	}
	for _, raw := range bad {
		if _, err := ParseRepoURL(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestParseBranch(t *testing.T) {
	got, err := ParseBranch("")
	if err != nil || got != "" {
		t.Fatalf("empty branch: %q %v", got, err)
	}
	got, err = ParseBranch("feature/one")
	if err != nil || got != "feature/one" {
		t.Fatalf("feature branch: %q %v", got, err)
	}
	for _, raw := range []string{"-main", "main;rm", "main..old", "main branch"} {
		if _, err := ParseBranch(raw); err == nil {
			t.Fatalf("accepted branch %q", raw)
		}
	}
}
