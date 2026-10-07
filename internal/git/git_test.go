package git

import "testing"

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want Status
	}{
		{
			name: "branch with upstream",
			out:  "# branch.oid 0ea0194abc\x00# branch.head main\x00# branch.upstream origin/main\x00# branch.ab +2 -1\x00",
			want: Status{Head: "main", OID: "0ea0194abc", Upstream: "origin/main", Ahead: 2, Behind: 1},
		},
		{
			name: "no upstream",
			out:  "# branch.oid 0ea0194abc\x00# branch.head feature/x\x00",
			want: Status{Head: "feature/x", OID: "0ea0194abc"},
		},
		{
			name: "detached",
			out:  "# branch.oid 0ea0194abc\x00# branch.head (detached)\x00",
			want: Status{OID: "0ea0194abc"},
		},
		{
			name: "before first commit",
			out:  "# branch.oid (initial)\x00# branch.head main\x00",
			want: Status{Head: "main"},
		},
		{
			name: "file entries are ignored",
			out:  "# branch.oid abc\x00# branch.head main\x001 .M N... 100644 100644 100644 abc abc main.go\x00? new file.txt\x00",
			want: Status{Head: "main", OID: "abc"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseStatus(tt.out); got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRepoName(t *testing.T) {
	main := Repo{Root: "/code/overlook", GitDir: "/code/overlook/.git", CommonDir: "/code/overlook/.git"}
	if main.IsWorktree() || main.Name() != "overlook" {
		t.Errorf("main repo: worktree=%v name=%q", main.IsWorktree(), main.Name())
	}
	wt := Repo{Root: "/code/overlook-wt/fix", GitDir: "/code/overlook/.git/worktrees/fix", CommonDir: "/code/overlook/.git"}
	if !wt.IsWorktree() || wt.Name() != "overlook" || wt.WorktreeName() != "fix" {
		t.Errorf("worktree: worktree=%v name=%q wt=%q", wt.IsWorktree(), wt.Name(), wt.WorktreeName())
	}
}
