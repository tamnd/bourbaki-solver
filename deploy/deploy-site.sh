#!/bin/sh
# Build the site of the corpus tamnd/bourbaki and publish it at https://bourbaki.tamnd.com.
#
#   deploy-site.sh [-f]
#
# The pages go to the R2 bucket bourbaki-site, gzipped, and the Worker in this
# directory serves them. Only the files whose bytes changed are uploaded, and a
# page the build no longer writes is deleted from the bucket. It does nothing
# when the corpus and the toolchain are at the commits of the last deploy; -f
# deploys again regardless.
#
# Run it from cron on a build host with git, Go, Node, jq and flock. A full
# build sets every formula of the corpus through KaTeX and takes a while, so
# every 30 minutes is plenty. $BOURBAKI_SITE/cloudflare.env, at mode 600, sets
#   CLOUDFLARE_API_TOKEN   Workers Scripts and Workers R2 Storage edit on the account, Workers Routes and DNS edit on tamnd.com
#   CLOUDFLARE_ACCOUNT_ID
# and may set
#   BOURBAKI_PUBLISH_FLAGS defaults to -allow-broken-math, which marks a formula KaTeX refuses rather than stopping
#   BOURBAKI_SOLVER_REF    the branch of tamnd/bourbaki-solver to build with, main by default
#   BOURBAKI_LOCAL_TOOLCHAIN=1  build with $BOURBAKI_SITE/toolchain as it is, for a tree copied there by hand
set -eu

dir=${BOURBAKI_SITE:-$HOME/bourbaki-site}
force=${1:-}
mkdir -p "$dir/bin"
exec 9>"$dir/lock"
flock -n 9 || { echo "another deploy is running"; exit 0; }

# A Go installed beside the site, for a host whose system Go is too old or absent.
[ -x "$dir/go/bin/go" ] && PATH="$dir/go/bin:$PATH"

set -a
. "$dir/cloudflare.env"
set +a
: "${CLOUDFLARE_ACCOUNT_ID:?}" "${CLOUDFLARE_API_TOKEN:?}"
flags=${BOURBAKI_PUBLISH_FLAGS:--allow-broken-math}
ref=${BOURBAKI_SOLVER_REF:-main}

fresh() { # fresh REPO DIR REF: a shallow checkout of REF
	if [ -d "$2/.git" ]; then
		git -C "$2" fetch -q --depth 1 origin "$3"
		git -C "$2" reset -q --hard FETCH_HEAD
	else
		git clone -q --depth 1 --branch "$3" "https://github.com/$1.git" "$2"
	fi
	git -C "$2" rev-parse HEAD
}
corpus=$(fresh tamnd/bourbaki "$dir/corpus" main)
if [ "${BOURBAKI_LOCAL_TOOLCHAIN:-}" = 1 ]; then
	toolchain=local-$(find "$dir/toolchain" -name '*.go' -o -name '*.css' -o -name '*.js' | sort | xargs cat | sha256sum | cut -c1-12)
else
	toolchain=$(fresh tamnd/bourbaki-solver "$dir/toolchain" "$ref")
fi

state="$corpus $toolchain"
if [ "$force" != "-f" ] && [ -f "$dir/deployed" ] && [ "$(cat "$dir/deployed")" = "$state" ]; then
	exit 0
fi
echo "$(date -u +%FT%TZ) deploy bourbaki $corpus, bourbaki-solver $toolchain"

(cd "$dir/toolchain" && go build -trimpath -o "$dir/bin/bourbaki" ./cmd/bourbaki)
# The directory is kept between runs, so the upload below sends only what
# changed. A page the corpus no longer has stays on disk from an older build
# until -f starts from an empty one, and the sync then deletes it from the bucket.
[ "$force" = "-f" ] && rm -rf "$dir/site"
# shellcheck disable=SC2086
(cd "$dir/corpus" && "$dir/bin/bourbaki" publish -gzip -out "$dir/site" $flags)

# R2 speaks S3, and the S3 key of an API token is its id with the SHA-256 of its
# value as the secret.
if [ ! -x "$dir/bin/rclone" ]; then
	curl -fsSL -o "$dir/rclone.zip" https://downloads.rclone.org/rclone-current-linux-amd64.zip
	unzip -oqj "$dir/rclone.zip" '*/rclone' -d "$dir/bin" && rm "$dir/rclone.zip"
fi
id=$(curl -fsS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" https://api.cloudflare.com/client/v4/user/tokens/verify | jq -r .result.id)
export RCLONE_CONFIG_R2_TYPE=s3 RCLONE_CONFIG_R2_PROVIDER=Cloudflare RCLONE_CONFIG_R2_REGION=auto RCLONE_CONFIG_R2_NO_CHECK_BUCKET=true
export RCLONE_CONFIG_R2_ENDPOINT="https://$CLOUDFLARE_ACCOUNT_ID.r2.cloudflarestorage.com"
export RCLONE_CONFIG_R2_ACCESS_KEY_ID="$id"
RCLONE_CONFIG_R2_SECRET_ACCESS_KEY=$(printf %s "$CLOUDFLARE_API_TOKEN" | sha256sum | cut -d' ' -f1)
export RCLONE_CONFIG_R2_SECRET_ACCESS_KEY
"$dir/bin/rclone" sync "$dir/site" r2:bourbaki-site --checksum --fast-list --transfers 32 --checkers 32 --stats-one-line --stats 60s

(cd "$dir/toolchain/deploy" && npx --yes wrangler@4 deploy)
echo "$state" >"$dir/deployed"
