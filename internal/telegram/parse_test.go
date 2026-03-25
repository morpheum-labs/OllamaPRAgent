package telegram

import "testing"

func TestParseReviewTarget(t *testing.T) {
	t.Parallel()
	repo, pr, err := ParseReviewTarget("42", "acme/app")
	if err != nil || repo != "acme/app" || pr != 42 {
		t.Fatalf("default repo: got %q %d %v", repo, pr, err)
	}
	repo, pr, err = ParseReviewTarget("acme/app#7", "")
	if err != nil || repo != "acme/app" || pr != 7 {
		t.Fatalf("hash form: got %q %d %v", repo, pr, err)
	}
	repo, pr, err = ParseReviewTarget("acme/app 99", "")
	if err != nil || repo != "acme/app" || pr != 99 {
		t.Fatalf("two tokens: got %q %d %v", repo, pr, err)
	}
	_, _, err = ParseReviewTarget("42", "")
	if err == nil {
		t.Fatal("expected error without default repo")
	}
}
