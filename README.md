# jot

Quick notes and todos from the terminal, stored as plain Markdown files you own.

You are in the middle of something and need to write a thing down. Opening an app breaks your focus. `jot` is one command, one line, and you are back to work. Later you can list, finish, and search everything you wrote, or open the files in any editor.

```console
$ jot standup moved to 10:30
✓ noted
$ jot t renew passport
✓ added todo #1
$ jot t review PR from Sam '#work'
✓ added todo #2
$ jot t fix login redirect '#work'
✓ added todo #3

$ jot ls
  1 renew passport
  2 review PR from Sam #work
  3 fix login redirect #work

$ jot done 1
✓ renew passport

$ jot ls '#work'
  1 review PR from Sam #work
  2 fix login redirect #work

$ jot find sam
todo       [ ] review PR from Sam #work

$ jot today
Saturday, October 24 2026
  09:12 standup moved to 10:30
```

## Install

With Go 1.22 or newer:

```sh
go install github.com/cipherhex09/jot@latest
```

Or download a binary for macOS, Linux or Windows from the [releases page](https://github.com/cipherhex09/jot/releases), make it executable and put it on your `PATH`:

```sh
chmod +x jot-darwin-arm64
mv jot-darwin-arm64 /usr/local/bin/jot
```

To build from source:

```sh
git clone https://github.com/cipherhex09/jot
cd jot
go build -o jot .
```

There are no dependencies outside the Go standard library.

## Commands

| Command | What it does |
|---|---|
| `jot <text>` | Add a timestamped note to today's file |
| `jot -` | Read notes from stdin, one per line (`pbpaste \| jot -`) |
| `jot t <text>` | Add a todo (also `todo` or `+`) |
| `jot` or `jot ls [filter]` | List open todos, optionally only ones containing a tag or word |
| `jot done 2 4` | Finish todos by number. Commas work too: `jot x 2,4` |
| `jot rm 3` | Delete a todo without keeping it |
| `jot clean` | Move finished todos into `done.md` with the date |
| `jot today` | Show today's notes |
| `jot log [days]` | Show notes from the last N days (default 7) |
| `jot find <words>` | Case-insensitive search across every file. All words must match |
| `jot edit [todo\|done]` | Open today's notes, or a list, in `$VISUAL` / `$EDITOR` |
| `jot path` | Print the notes folder |

Todo numbers are always the ones `jot ls` shows, so they stay short. When you finish #2, the todo that was #3 becomes #2.

If a note starts with a command word, put `--` first: `jot -- ls is broken on staging`.

Words that start with `#` are tags. Quote them (`'#work'`) so your shell does not read them as a comment.

## Where your notes live

Everything is in `~/.jot` unless you set `JOT_DIR`:

```
~/.jot/
  todo.md              - [ ] open and - [x] finished todos
  done.md              todos moved out by `jot clean`
  notes/2026-10-24.md  one file per day
```

A day file looks like this:

```markdown
# Saturday, October 24 2026

- 09:12 standup moved to 10:30
- 14:40 the staging password is in the team vault, not the wiki
```

Point `JOT_DIR` at a synced folder (iCloud Drive, Dropbox, a git repo, an Obsidian vault) and your notes follow you. Edits to `todo.md` are written to a temp file and renamed, so a crash never leaves a half written list.

Set `NO_COLOR=1` to turn off colour. Output piped to another program is never coloured.

## License

MIT
