package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Store is a folder of plain Markdown files:
//
//	todo.md            open and finished todos, one "- [ ] text" line each
//	done.md            todos moved out by `jot clean`
//	notes/YYYY-MM-DD.md  one file per day, "- HH:MM text" lines
//
// Everything is append-friendly and readable without jot.
type Store struct {
	Dir string
	Now func() time.Time
}

type Todo struct {
	N    int // 1-based position among open todos, 0 when done
	Text string
	Done bool
	line int // index into the raw file lines
}

type Match struct {
	File string
	Line int
	Text string
}

func DefaultDir() string {
	if d := os.Getenv("JOT_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".jot"
	}
	return filepath.Join(home, ".jot")
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Store) todoPath() string { return filepath.Join(s.Dir, "todo.md") }
func (s *Store) donePath() string { return filepath.Join(s.Dir, "done.md") }
func (s *Store) notesDir() string { return filepath.Join(s.Dir, "notes") }

func (s *Store) DayPath(t time.Time) string {
	return filepath.Join(s.notesDir(), t.Format("2006-01-02")+".md")
}

func (s *Store) AddNote(text string) error {
	text = oneLine(text)
	if text == "" {
		return errors.New("empty note")
	}
	t := s.now()
	path := s.DayPath(t)
	header := ""
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		header = "# " + t.Format("Monday, January 2 2006") + "\n\n"
	}
	return appendFile(path, header+"- "+t.Format("15:04")+" "+text+"\n")
}

func (s *Store) AddTodo(text string) (int, error) {
	text = oneLine(text)
	if text == "" {
		return 0, errors.New("empty todo")
	}
	if err := appendFile(s.todoPath(), "- [ ] "+text+"\n"); err != nil {
		return 0, err
	}
	todos, err := s.Todos()
	if err != nil {
		return 0, err
	}
	return countOpen(todos), nil
}

// Todos returns every todo in file order. Open ones are numbered 1..n so the
// numbers people type match what `jot ls` printed.
func (s *Store) Todos() ([]Todo, error) {
	lines, err := readLines(s.todoPath())
	if err != nil {
		return nil, err
	}
	var out []Todo
	n := 0
	for i, l := range lines {
		text, done, ok := parseTodo(l)
		if !ok {
			continue
		}
		t := Todo{Text: text, Done: done, line: i}
		if !done {
			n++
			t.N = n
		}
		out = append(out, t)
	}
	return out, nil
}

// Done marks open todos finished by their `jot ls` numbers.
func (s *Store) Done(nums []int) ([]Todo, error) {
	return s.editTodos(nums, func(lines []string, t Todo) []string {
		lines[t.line] = "- [x] " + t.Text
		return lines
	})
}

// Remove deletes open todos by number without keeping them anywhere.
func (s *Store) Remove(nums []int) ([]Todo, error) {
	return s.editTodos(nums, func(lines []string, t Todo) []string {
		lines[t.line] = "\x00"
		return lines
	})
}

func (s *Store) editTodos(nums []int, edit func([]string, Todo) []string) ([]Todo, error) {
	lines, err := readLines(s.todoPath())
	if err != nil {
		return nil, err
	}
	todos, err := s.Todos()
	if err != nil {
		return nil, err
	}
	byN := map[int]Todo{}
	for _, t := range todos {
		if !t.Done {
			byN[t.N] = t
		}
	}
	var hit []Todo
	for _, n := range nums {
		t, ok := byN[n]
		if !ok {
			return nil, fmt.Errorf("no open todo #%d (run `jot ls`)", n)
		}
		hit = append(hit, t)
	}
	for _, t := range hit {
		lines = edit(lines, t)
	}
	kept := lines[:0]
	for _, l := range lines {
		if l != "\x00" {
			kept = append(kept, l)
		}
	}
	return hit, writeLines(s.todoPath(), kept)
}

// Clean moves finished todos into done.md with today's date.
func (s *Store) Clean() (int, error) {
	lines, err := readLines(s.todoPath())
	if err != nil {
		return 0, err
	}
	var keep []string
	var moved strings.Builder
	n := 0
	stamp := s.now().Format("2006-01-02")
	for _, l := range lines {
		if text, done, ok := parseTodo(l); ok && done {
			moved.WriteString("- [x] " + text + " (" + stamp + ")\n")
			n++
			continue
		}
		keep = append(keep, l)
	}
	if n == 0 {
		return 0, nil
	}
	if err := appendFile(s.donePath(), moved.String()); err != nil {
		return 0, err
	}
	return n, writeLines(s.todoPath(), keep)
}

// Days returns note files for the last n days, newest last.
func (s *Store) Days(n int) ([]string, error) {
	var out []string
	t := s.now()
	for i := n - 1; i >= 0; i-- {
		p := s.DayPath(t.AddDate(0, 0, -i))
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// Find does a case-insensitive search across every .md file in the store.
// All terms must appear on the line.
func (s *Store) Find(terms []string) ([]Match, error) {
	var files []string
	err := filepath.WalkDir(s.Dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return filepath.SkipAll
			}
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	for i := range terms {
		terms[i] = strings.ToLower(terms[i])
	}
	var out []Match
	for _, f := range files {
		lines, err := readLines(f)
		if err != nil {
			return nil, err
		}
		for i, l := range lines {
			low := strings.ToLower(l)
			all := true
			for _, t := range terms {
				if !strings.Contains(low, t) {
					all = false
					break
				}
			}
			if all && strings.TrimSpace(l) != "" && !strings.HasPrefix(l, "# ") {
				rel, _ := filepath.Rel(s.Dir, f)
				out = append(out, Match{File: rel, Line: i + 1, Text: l})
			}
		}
	}
	return out, nil
}

func parseTodo(l string) (text string, done bool, ok bool) {
	l = strings.TrimRight(l, " \t")
	switch {
	case strings.HasPrefix(l, "- [ ] "):
		return l[6:], false, true
	case strings.HasPrefix(l, "- [x] "), strings.HasPrefix(l, "- [X] "):
		return l[6:], true, true
	}
	return "", false, false
}

func countOpen(ts []Todo) int {
	n := 0
	for _, t := range ts {
		if !t.Done {
			n++
		}
	}
	return n
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out, sc.Err()
}

// writeLines replaces a file atomically so a crash never leaves half a todo list.
func writeLines(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".jot-*")
	if err != nil {
		return err
	}
	body := strings.Join(lines, "\n")
	if len(lines) > 0 {
		body += "\n"
	}
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func appendFile(path, s string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(s); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
