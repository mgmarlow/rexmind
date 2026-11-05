# rexmind

A Procfile process manager in the style of [overmind](https://github.com/DarthSim/overmind), built on Rex instead of tmux.

Each process runs in its own Rex block in a session named `rexmind:<project dir>`. The Rex server holds the state, so there is no control socket or daemon. Closing the terminal that ran `start` leaves the processes running.

## Install

```sh
go install github.com/mgmarlow/rexmind/cmd/rexmind@latest
```

Requires a local Rex server and the `rex` CLI on `PATH`.

## Usage

```sh
rexmind start             # Procfile.dev, or Procfile; Ctrl-C stops everything
rexmind start -l web,css  # only some processes
rexmind status
rexmind restart web       # same block, scrollback kept
rexmind stop worker       # stop the process, keep its block
rexmind connect web       # show the block in the Rex app, zoomed
rexmind kill              # stop everything from another terminal
```

| overmind | rexmind |
| --- | --- |
| `overmind start` | `rexmind start` |
| `overmind connect web` | `rexmind connect web` |
| `overmind restart web` | `rexmind restart web` |
| `overmind stop web` | `rexmind stop web` |
| `overmind kill` | `rexmind kill` |
| `overmind status` | `rexmind status` |

### Environment

Env files load as overmind loads them, with the same parser: `~/.overmind.env`, `.overmind.env`, `.env` (skipped if `OVERMIND_SKIP_ENV` is set), then each file in `OVERMIND_ENV`. Later files win, and a file that fails to parse is skipped with a warning. Each process gets the environment `rexmind start` ran with, then those files, then `PORT`, as with overmind. Rex starts blocks with a login-shell environment that skips `~/.zshrc`, where tools like asdf, nvm and pnpm often set `PATH`. So `start` saves its environment to a per-user temp file (mode 0600, removed by `kill`) for each block's wrapper to load. Rex's per-block variables (`REX_*`, `TERM`) still come from the block. `PORT` is `-p` (default 5000) plus `-P` (default 100) times the process's position in the Procfile, so `-l` does not shift ports.

## How it works

Each block's program is `rexmind _exec`, a small wrapper that runs the real command in its own process group and makes it the terminal's foreground group, so typing and Ctrl-C reach it. rexmind finds the wrapper's PID through the block's `process` method and controls it by signal:

| Signal | Effect |
| --- | --- |
| `SIGUSR2` | restart the process (or start it if stopped) |
| `SIGUSR1` | stop the process; the block stays |
| `SIGHUP`, `SIGTERM`, `SIGINT` | stop the process and exit, closing the block |

Stopping sends `SIGTERM` to the process group, then `SIGKILL` to any members still running after `-t` (default 5s). `-t` is given to `start` and applies to every later stop, restart and kill. When a process exits on its own, background jobs it left in its group are stopped the same way. Rex sends `SIGHUP` when a block or session closes, so hangup always means shutdown.

The wrapper reports state with OSC 7501 program status sequences, which Rex shows natively and `rexmind status` reads back.

## Not yet supported

- Combined, prefixed log output (`overmind echo`)
- Formation (`-m web=2`), `can-die`, auto-restart
- Remote Rex servers (signals are sent to local PIDs)
- Two projects whose directories share a name (their session labels collide)
