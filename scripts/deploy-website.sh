#!/usr/bin/env bash
# Build the website and deploy it to Cloudflare Pages.
#
# Needs CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN, taken from the
# environment or, when unset, from .env at the repository root.
#
# Usage: scripts/deploy-website.sh [--branch NAME] [--no-build]
# Env:   CF_PAGES_PROJECT (default: techlog), CF_PAGES_BRANCH (default: main)
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
project="${CF_PAGES_PROJECT:-techlog}"
branch="${CF_PAGES_BRANCH:-main}"
build=1

while [ $# -gt 0 ]; do
  case "$1" in
    --branch) branch="${2:?--branch needs a value}"; shift 2 ;;
    --no-build) build=0; shift ;;
    -h|--help) sed -n '2,9p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done

# Fill only the variables that are not already set; the environment wins.
if [ -f "$root/.env" ]; then
  for var in CLOUDFLARE_ACCOUNT_ID CLOUDFLARE_API_TOKEN; do
    if [ -z "${!var:-}" ]; then
      line="$(grep -E "^(export[[:space:]]+)?$var=" "$root/.env" | tail -n 1 || true)"
      value="${line#*=}"
      value="${value%\"}"; value="${value#\"}"; value="${value%\'}"; value="${value#\'}"
      [ -n "$value" ] && export "$var=$value"
    fi
  done
fi

for var in CLOUDFLARE_ACCOUNT_ID CLOUDFLARE_API_TOKEN; do
  [ -n "${!var:-}" ] || { echo "$var is not set (environment or $root/.env)" >&2; exit 1; }
done

if [ "$build" = 1 ]; then
  (cd "$root/website" && npm run build && npm run check)
fi
[ -f "$root/website/site/index.html" ] || { echo "website/site is empty; run without --no-build" >&2; exit 1; }

wrangler=(npx --yes wrangler@4)

# Create the Pages project on the first deploy.
if ! "${wrangler[@]}" pages project list 2>/dev/null | grep -qE "[│|][[:space:]]*$project[[:space:]]*[│|]"; then
  echo "creating Pages project $project"
  "${wrangler[@]}" pages project create "$project" --production-branch main
fi

"${wrangler[@]}" pages deploy "$root/website/site" \
  --project-name "$project" --branch "$branch" --commit-dirty=true
