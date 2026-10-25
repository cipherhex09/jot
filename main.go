// jot is a tiny command line notebook: quick notes, todos and search over
// plain Markdown files you own.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var version = "dev"

const usage = `jot: quick notes and todos in plain Markdown

Usage:
  jot <text>             add a note to today's file
  jot -                  add a note from stdin (one note per line)
  jot t <text>           add a todo            (alias: todo, +)
  jot ls [#tag|word]     list open todos       (alias: l)
  jot done <n>...        finish todos by number (alias: x)
  jot rm <n>...          delete todos by number
  jot clean              move finished todos to done.md
  jot today              show today's notes
  jot log [days]         show notes from the last N days (default 7)
  jot find <words>...    search everything     (alias: f, /)
  jot edit [todo|done]   open today's notes (or a list) in $EDITOR
  jot path               print the notes folder
  jot version

Notes live in ~/.jot (set JOT_DIR to move them). Set NO_COLOR to turn off colour.
`

func main() {
	s := &Store{Dir: DefaultDir()}
	if err := run(s, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "jot:", err)
		os.Exit(1)
	}
}

func run(s *Store, args []string, stdin io.Reader, out io.Writer) error {
	c := newColors(out)
	if len(args) == 0 {
		return list(s, nil, out, c)
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "-h", "--help", "help":
		fmt.Fprint(out, usage)
		return nil
	case "-v", "--version", "version":
		fmt.Fprintln(out, "jot", version)
		return nil
	case "t", "todo", "+":
		n, err := s.AddTodo(strings.Join(rest, " "))
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s added todo %s\n", c.green("✓"), c.bold("#"+strconv.Itoa(n)))
		return nil
	case "ls", "l", "list":
		return list(s, rest, out, c)
	case "done", "x":
		nums, err := parseNums(rest)
		if err != nil {
			return err
		}
		hit, err := s.Done(nums)
		if err != nil {
			return err
		}
		for _, t := range hit {
			fmt.Fprintf(out, "%s %s\n", c.green("✓"), c.dim(t.Text))
		}
		return nil
	case "rm":
		nums, err := parseNums(rest)
		if err != nil {
			return err
		}
		hit, err := s.Remove(nums)
		if err != nil {
			return err
		}
		for _, t := range hit {
			fmt.Fprintf(out, "%s %s\n", c.red("✗"), c.dim(t.Text))
		}
		return nil
	case "clean":
		n, err := s.Clean()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "moved %d finished todo%s to done.md\n", n, plural(n))
		return nil
	case "today":
		return showDays(s, 1, out, c)
	case "log":
		days := 7
		if len(rest) > 0 {
			d, err := strconv.Atoi(rest[0])
			if err != nil || d < 1 {
				return fmt.Errorf("log takes a number of days, got %q", rest[0])
			}
			days = d
		}
		return showDays(s, days, out, c)
	case "find", "f", "/":
		if len(rest) == 0 {
			return errors.New("find what? try `jot find dentist`")
		}
		return find(s, rest, out, c)
	case "edit", "e":
		return edit(s, rest)
	case "path":
		fmt.Fprintln(out, s.Dir)
		return nil
	case "-":
		return notesFromStdin(s, stdin, out, c)
	}
	// Anything else is a note. "--" lets a note start with a command word.
	if cmd == "--" {
		args = rest
	}
	if err := s.AddNote(strings.Join(args, " ")); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s noted\n", c.green("✓"))
	return nil
}

func list(s *Store, filter []string, out io.Writer, c colors) error {
	todos, err := s.Todos()
	if err != nil {
		return err
	}
	shown := 0
	for _, t := range todos {
		if t.Done || !matchesAll(t.Text, filter) {
			continue
		}
		fmt.Fprintf(out, "%s %s\n", c.dim(fmt.Sprintf("%3d", t.N)), c.tags(t.Text))
		shown++
	}
	if shown == 0 {
		if len(filter) > 0 {
			fmt.Fprintln(out, c.dim("no open todos match"))
		} else {
			fmt.Fprintln(out, c.dim("nothing to do. add one with `jot t <text>`"))
		}
	}
	return nil
}

func showDays(s *Store, n int, out io.Writer, c colors) error {
	files, err := s.Days(n)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Fprintln(out, c.dim("no notes yet. add one with `jot <text>`"))
		return nil
	}
	for i, f := range files {
		lines, err := readLines(f)
		if err != nil {
			return err
		}
		if i > 0 {
			fmt.Fprintln(out)
		}
		for _, l := range lines {
			switch {
			case strings.HasPrefix(l, "# "):
				fmt.Fprintln(out, c.bold(l[2:]))
			case strings.HasPrefix(l, "- ") && len(l) > 8 && l[4] == ':':
				fmt.Fprintf(out, "  %s %s\n", c.dim(l[2:7]), c.tags(l[8:]))
			case strings.TrimSpace(l) != "":
				fmt.Fprintln(out, "  "+c.tags(l))
			}
		}
	}
	return nil
}

func find(s *Store, terms []string, out io.Writer, c colors) error {
	ms, err := s.Find(terms)
	if err != nil {
		return err
	}
	if len(ms) == 0 {
		fmt.Fprintln(out, c.dim("no matches"))
		return nil
	}
	for _, m := range ms {
		name := strings.TrimSuffix(strings.TrimPrefix(m.File, "notes/"), ".md")
		fmt.Fprintf(out, "%s %s\n", c.dim(fmt.Sprintf("%-10s", name)), c.highlight(strings.TrimPrefix(m.Text, "- "), terms))
	}
	return nil
}

func edit(s *Store, args []string) error {
	path := s.DayPath(s.now())
	if len(args) > 0 {
		switch args[0] {
		case "todo", "t":
			path = s.todoPath()
		case "done":
			path = s.donePath()
		default:
			return fmt.Errorf("edit takes todo or done, got %q", args[0])
		}
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := appendFile(path, ""); err != nil {
			return err
		}
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	// $EDITOR is often "code -w" or "subl -w", so let the shell split it.
	cmd := exec.Command("sh", "-c", editor+` "$1"`, "sh", path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func notesFromStdin(s *Store, r io.Reader, out io.Writer, c colors) error {
	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		if err := s.AddNote(sc.Text()); err != nil {
			return err
		}
		n++
	}
	if err := sc.Err(); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s noted %d line%s\n", c.green("✓"), n, plural(n))
	return nil
}

func parseNums(args []string) ([]int, error) {
	if len(args) == 0 {
		return nil, errors.New("which todo? pass its number from `jot ls`")
	}
	var out []int
	for _, a := range args {
		for _, part := range strings.Split(a, ",") {
			if part == "" {
				continue
			}
			n, err := strconv.Atoi(strings.TrimPrefix(part, "#"))
			if err != nil || n < 1 {
				return nil, fmt.Errorf("%q is not a todo number", part)
			}
			out = append(out, n)
		}
	}
	return out, nil
}

func matchesAll(text string, terms []string) bool {
	low := strings.ToLower(text)
	for _, t := range terms {
		if !strings.Contains(low, strings.ToLower(t)) {
			return false
		}
	}
	return true
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

type colors struct{ on bool }

func newColors(w io.Writer) colors {
	if os.Getenv("NO_COLOR") != "" {
		return colors{}
	}
	f, ok := w.(*os.File)
	if !ok {
		return colors{}
	}
	st, err := f.Stat()
	return colors{on: err == nil && st.Mode()&os.ModeCharDevice != 0}
}

func (c colors) wrap(code, s string) string {
	if !c.on {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (c colors) bold(s string) string  { return c.wrap("1", s) }
func (c colors) dim(s string) string   { return c.wrap("2", s) }
func (c colors) green(s string) string { return c.wrap("32", s) }
func (c colors) red(s string) string   { return c.wrap("31", s) }

var tagRe = regexp.MustCompile(`(^|\s)(#[\p{L}\p{N}_/-]+)`)

func (c colors) tags(s string) string {
	if !c.on {
		return s
	}
	return tagRe.ReplaceAllString(s, "$1\x1b[36m$2\x1b[0m")
}

func (c colors) highlight(s string, terms []string) string {
	if !c.on {
		return s
	}
	for _, t := range terms {
		re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(t))
		if err != nil {
			continue
		}
		s = re.ReplaceAllString(s, "\x1b[1;33m$0\x1b[0m")
	}
	return s
}
