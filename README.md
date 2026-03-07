# newbrew

`newbrew` is a cheerful little terminal companion for keeping an eye on freshly merged Homebrew formulae.

It looks up recent `new formula` pull requests in [`Homebrew/homebrew-core`](https://github.com/Homebrew/homebrew-core), pulls out useful details like descriptions and homepages, and presents everything in a tidy terminal UI so you can browse without leaving your shell.

## What it does

- fetches recently merged Homebrew PRs labelled `new formula`
- finds the added formula files in each PR
- extracts each formula's description and homepage
- shows the results in a searchable terminal list
- opens the selected formula homepage in your default browser

In short: if you like discovering shiny new CLI tools and packages, `newbrew` is here to hand you a fresh mug of them.

## Install

### Homebrew

```bash
brew install matt-riley/tools/newbrew
```

### From source

```bash
git clone git@github.com:matt-riley/newbrew.git
cd newbrew/cli
go run .
```

If you want a standalone binary instead:

```bash
go build -o newbrew
./newbrew
```

## How to use it

Run:

```bash
newbrew
```

That launches the TUI and loads recent Homebrew formula additions.

If you have a GitHub token handy, you can pass it in to get nicer API rate limits:

```bash
GITHUB_TOKEN=your_token_here newbrew
```

`newbrew` also keeps a short-lived cache in your user cache directory, so repeated launches are snappy and friendly to the GitHub API. Cached results stay fresh for about 10 minutes, and you can always refresh manually from inside the app.

## Controls

Once the app is open, these are the main moves:

| Key | Action |
| --- | --- |
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Open the selected formula homepage |
| `r` | Refresh the list |
| `q` / `Ctrl+C` | Quit |

The list also includes built-in filtering controls in the footer, so you can quickly narrow things down when something catches your eye.

## What you'll see

Each list entry includes:

- the pull request title
- the formula description
- a homepage link you can open with `Enter`

If a cached result is being shown, the UI lets you know. If there is nothing recent to show, `newbrew` will say so plainly instead of pretending otherwise. Honest software! What a treat.

## A few helpful notes

- On Linux, homepage links are opened with `xdg-open`.
- On macOS, homepage links are opened with `open`.
- If a formula is missing a description or homepage in the source file, `newbrew` shows a fallback instead of exploding dramatically.
- The app currently focuses on recent additions to `Homebrew/homebrew-core`, not the full historical catalog.

## Development

The Go CLI lives in `cli/`.

Useful commands:

```bash
cd cli
go test ./...
go build .
```

## Why this exists

Because browsing new Homebrew formulae should feel a little more fun than manually clicking through search results.

May your taps be tidy, your caches warm, and your terminal full of delightful new tools.
