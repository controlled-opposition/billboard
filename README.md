# billboard

> [!WARNING]
> billboard is in a very early state. Commands, flags, JSON fields, and exit codes can change without notice. It is tested against one student account at William & Mary.

billboard is named after the famous Super Mario character [Bill Board.](https://www.mariowiki.com/Bill_Board)

`bb` is a command-line client for [Blackboard Learn Ultra at William & Mary](https://blackboard.wm.edu), modeled on GitHub's `gh`. It is built mainly for AI agents that work from a shell, but humans can use it too. It reads your courses, due dates, and course materials, more functionality will be added later.

## Workflow

Run `bb` with Nix, without installing it:

```sh
nix run github:controlled-opposition/billboard -- --help
```

To put `bb` on your `PATH`, install it into your profile:

```sh
nix profile install github:controlled-opposition/billboard
```

1. Log in once. `bb auth login` opens Chrome for your NetID and MFA, then stores the session in the macOS Keychain.

   ```sh
   bb auth login
   ```

2. Read from Blackboard. Every command reuses the stored session and renews it in the background when it can.

   ```sh
   bb course list                              # current courses
   bb due list --until 7d                      # what is due this week, with submission status
   bb due list --pending                       # what is due and not submitted yet
   bb content list math101 --files-only        # course materials with files
   bb content download math101 _20001_1 --dir ~/bb
   bb due list --json title,due,course         # JSON for scripts and agents
   ```

3. When a command exits with code 4, the session cannot be renewed. Run `bb auth login` again.

`bb` never prompts. Agents branch on the exit code, which every `bb <command> --help` lists, and on the `--json` output.

## Contributing

Read [`AGENTS.md`](AGENTS.md) for the build, the rules the code depends on, and the commit format. The design is in [`docs/design.md`](docs/design.md).
