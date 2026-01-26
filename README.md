# Loki

A minimal, educational Git-style version control system written in Go. Loki demonstrates the core ideas behind Git: object storage, staging, commits, and a simple CLI.

---

## Features

- **init**: Initialize a new Loki repository
- **add <files>**: Stage files for commit
- **commit -m "message"**: Commit staged files
- **status**: Show staged files
- **log**: Show commit log
- Real object storage: blobs, trees, and commits (Git-style)

---

## Quick Start

1. **Clone or download this repository**
2. **Build the CLI:**
   ```sh
   go build -o loki ./cmd/loki
   ```
3. **Run commands:**
   ```sh
   ./loki init
   ./loki add myfile.txt
   ./loki commit -m "first commit"
   ./loki status
   ./loki log
   ```

**Optional:**
To run `loki` from anywhere, move it to your PATH:
```sh
sudo mv loki /usr/local/bin/
```

---

## How It Works

- **Blobs**: Store file contents (content-addressed, like Git)
- **Trees**: Store directory structure and file metadata
- **Commits**: Store project history and point to tree objects
- All objects are hashed (SHA-1) and stored in `.loki/objects/` using a split directory structure

---

## Project Structure

```
loki/
├── cmd/
│   └── loki/
│       └── main.go                # CLI entry point
│
├── internal/
│   ├── commands/                  # Command handlers (init, add, commit, ...)
│   ├── core/                      # Core repository logic (index, repo, hash)
│   ├── models/                    # Data structures (blob, tree, commit)
│   └── storage/                   # Object storage (interface, file impl)
│
├── .gitignore
├── go.mod
├── README.md
└── project_structure.md
```

For a detailed explanation, see [`docs/architecture.md`](docs/architecture.md).

---

## Example Workflow

```sh
./loki init
# Add files to staging
./loki add main.go
./loki add README.md
# Commit staged files
./loki commit -m "Initial commit"
# Check status
./loki status
# View commit log
./loki log
```

---

## Contributing

Contributions, bug reports, and questions are welcome! Please open an issue or pull request.

---
