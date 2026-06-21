# FirstByte

A self-hostable daily news digest — delivered to your inbox every morning.

Aggregates RSS feeds from sources you care about and emails a curated digest.

## Quick Start

Copy config and secrets:

```bash
cp config.example.yaml config.yaml
cp .env.example .env
```

Edit **config.yaml** — add your RSS sources and email settings.

Edit **.env** — set `SMTP_USER` (Gmail address) and `SMTP_PASSWORD` (16-char App Password).

All config values can also be overridden via environment variables:

```bash
MAX_AGE_DAYS=3 EMAIL_TO=me@example.com ./firstbyte
```

Run:

```bash
go build -o firstbyte .
./firstbyte
```

## Deploy on a server

Cross-compile for Linux, copy files, set up cron:

```bash
GOOS=linux GOARCH=amd64 go build -o firstbyte .
scp firstbyte config.yaml .env template/email.html root@server:/opt/firstbyte/
```

On the server:

```bash
cd /opt/firstbyte && ./firstbyte
echo '0 8 * * * cd /opt/firstbyte && ./firstbyte' | crontab -
```

Override any setting at runtime without editing config.yaml:

```bash
MAX_AGE_DAYS=3 EMAIL_TO=me@example.com ./firstbyte
```

## Default Sources

- Hacker News
- GitHub Blog
- Lobsters
- OpenAI Blog
- Engadget
- Wired
- The Verge
- Ars Technica
- TNW
- Electrek
- TechCrunch

## Deploy with Docker

```bash
docker compose run --rm firstbyte
```
