# coda-cli

An open-source command-line tool for interacting with [Coda](https://coda.io) docs, pages, tables, and rows.

> **Not affiliated with or endorsed by Coda.io, Inc.**

Inspired by [jira-cli](https://github.com/ankitpokhrel/jira-cli) and [gh](https://github.com/cli/cli).

---

## Installation

### Go install

```sh
go install github.com/AfeefRazick/coda-cli/cmd/coda@latest
```

### Homebrew (coming soon)

```sh
brew install AfeefRazick/tap/coda
```

### Download binary

Download the latest release from the [releases page](https://github.com/AfeefRazick/coda-cli/releases).

---

## Authentication

Generate an API token at <https://coda.io/account> under **API settings**.

```sh
# Interactive prompt
coda auth login

# Pipe token directly
echo $TOKEN | coda auth login --with-token-stdin

# Or export for the session (takes precedence over saved config)
export CODA_API_TOKEN=your-token
```

Config is stored at `~/.config/coda-cli/config.yaml` with `0600` permissions.

---

## Usage

```
coda <command> [subcommand] [flags]
```

### Auth

```sh
coda auth login          # save a token
coda auth status         # verify token and show user
coda auth logout         # remove saved token
```

### Docs

```sh
coda docs list --all
coda docs list --query "roadmap" --owned
coda docs get <doc-id>
coda docs create --title "Launch Tracker"
coda docs update <doc-id> --title "New Title"
coda docs delete <doc-id> --yes
```

### Pages

```sh
coda pages list <doc-id>
coda pages get <doc-id> <page-id>
coda pages content <doc-id> <page-id> --format markdown
coda pages create <doc-id> --name Roadmap --content '<p>Hello</p>' --wait
coda pages update <doc-id> <page-id> --name "Updated Name" --wait
coda pages delete <doc-id> <page-id> --yes --wait
```

### Tables & Columns

```sh
coda tables list <doc-id>
coda tables get <doc-id> <table-id>
coda columns list <doc-id> <table-id>
coda columns get <doc-id> <table-id> <column-id>
```

### Rows

```sh
coda rows list <doc-id> <table-id> --limit 50
coda rows get <doc-id> <table-id> <row-id>
coda rows insert <doc-id> <table-id> --value Name="Quarterly plan" --value Status=Draft --wait
coda rows upsert <doc-id> <table-id> --key Name --value Name="Q3 plan" --value Status=Active --wait
coda rows update <doc-id> <table-id> <row-id> --value Status=Done --wait
coda rows delete <doc-id> <table-id> <row-id> --yes --wait
coda rows delete-many <doc-id> <table-id> --row i-123 --row i-456 --yes --wait
coda rows push-button <doc-id> <table-id> <row-id> <button-column-id> --wait
```

### Raw API access

```sh
coda api /docs --paginate
coda api /docs/<doc-id> --method PATCH --field title="New Title"
coda api /whoami
coda wait <request-id>
```

---

## Configuration

| Env var           | Description                                   |
|-------------------|-----------------------------------------------|
| `CODA_API_TOKEN`  | API token (takes precedence over config file) |
| `CODA_CONFIG_DIR` | Override config directory path                |

---

## Contributing

Contributions are welcome. Please open an issue first to discuss significant changes.

```sh
git clone https://github.com/AfeefRazick/coda-cli
cd coda-cli
make build
./bin/coda --help
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

---

## License

[MIT](LICENSE)
