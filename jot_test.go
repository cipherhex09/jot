package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	day := time.Date(2026, 3, 14, 9, 30, 0, 0, time.Local)
	return &Store{Dir: t.TempDir(), Now: func() time.Time { return day }}
}

func jot(t *testing.T, s *Store, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	if err := run(s, args, strings.NewReader(""), &out); err != nil {
		t.Fatalf("jot %v: %v", args, err)
	}
	return out.String()
}

func TestNotesGoToTodaysFile(t *testing.T) {
	s := testStore(t)
	jot(t, s, "call", "the", "dentist")
	jot(t, s, "--", "ls", "is a word too")

	b, err := os.ReadFile(filepath.Join(s.Dir, "notes", "2026-03-14.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# Saturday, March 14 2026\n\n- 09:30 call the dentist\n- 09:30 ls is a word too\n"
	if string(b) != want {
		t.Fatalf("got\n%q\nwant\n%q", b, want)
	}
	if out := jot(t, s, "today"); !strings.Contains(out, "09:30 call the dentist") {
		t.Fatalf("today output missing note:\n%s", out)
	}
}

func TestTodoLifecycle(t *testing.T) {
	s := testStore(t)
	jot(t, s, "t", "buy milk")
	jot(t, s, "t", "ship release #work")
	jot(t, s, "+", "reply to Sam #work")

	out := jot(t, s, "ls")
	for _, want := range []string{"1 buy milk", "2 ship release", "3 reply to Sam"} {
		if !strings.Contains(out, want) {
			t.Fatalf("ls missing %q:\n%s", want, out)
		}
	}
	if out := jot(t, s, "ls", "#work"); strings.Contains(out, "milk") {
		t.Fatalf("tag filter leaked:\n%s", out)
	}

	jot(t, s, "done", "2")
	out = jot(t, s, "ls")
	if strings.Contains(out, "ship release") || !strings.Contains(out, "2 reply to Sam") {
		t.Fatalf("numbers should close up after done:\n%s", out)
	}

	jot(t, s, "rm", "1")
	jot(t, s, "clean")
	todo, _ := os.ReadFile(filepath.Join(s.Dir, "todo.md"))
	if string(todo) != "- [ ] reply to Sam #work\n" {
		t.Fatalf("todo.md = %q", todo)
	}
	done, _ := os.ReadFile(filepath.Join(s.Dir, "done.md"))
	if string(done) != "- [x] ship release #work (2026-03-14)\n" {
		t.Fatalf("done.md = %q", done)
	}
}

func TestDoneRejectsUnknownNumber(t *testing.T) {
	s := testStore(t)
	jot(t, s, "t", "only one")
	var out bytes.Buffer
	if err := run(s, []string{"done", "1", "5"}, nil, &out); err == nil {
		t.Fatal("expected error for #5")
	}
	// A bad number must not half-apply the others.
	if got := jot(t, s, "ls"); !strings.Contains(got, "1 only one") {
		t.Fatalf("todo #1 should still be open:\n%s", got)
	}
}

func TestDoneAcceptsCommaLists(t *testing.T) {
	s := testStore(t)
	for _, x := range []string{"a", "b", "c"} {
		jot(t, s, "t", x)
	}
	jot(t, s, "x", "1,3")
	if got := jot(t, s, "ls"); strings.TrimSpace(got) != "1 b" {
		t.Fatalf("got %q", got)
	}
}

func TestFindSearchesEverything(t *testing.T) {
	s := testStore(t)
	jot(t, s, "Dentist moved to Friday")
	jot(t, s, "t", "book dentist parking")
	jot(t, s, "unrelated")

	out := jot(t, s, "find", "dentist")
	if !strings.Contains(out, "2026-03-14") || !strings.Contains(out, "todo") {
		t.Fatalf("find should hit notes and todos:\n%s", out)
	}
	if strings.Contains(out, "unrelated") {
		t.Fatalf("find matched too much:\n%s", out)
	}
	if out := jot(t, s, "f", "dentist", "parking"); strings.Contains(out, "Friday") {
		t.Fatalf("all terms must match:\n%s", out)
	}
}

func TestStdinNotes(t *testing.T) {
	s := testStore(t)
	var out bytes.Buffer
	if err := run(s, []string{"-"}, strings.NewReader("one\n\ntwo\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "noted 2 lines") {
		t.Fatalf("got %q", out.String())
	}
}

func TestLogSkipsMissingDays(t *testing.T) {
	s := testStore(t)
	jot(t, s, "today's note")
	day := s.Now()
	s.Now = func() time.Time { return day.AddDate(0, 0, 2) }
	jot(t, s, "two days later")
	out := jot(t, s, "log", "3")
	if !strings.Contains(out, "today's note") || !strings.Contains(out, "two days later") {
		t.Fatalf("log missing a day:\n%s", out)
	}
}
