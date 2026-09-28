#!/bin/sh
# Rebuilds examples/sample-project/.beads/issues.jsonl from nothing, with the
# bd on PATH, in an embedded project under a temporary directory. The issues
# are synthetic: a small web shop with epics, blocked work and closed work.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# Blank every Beads setting of the calling shell, so no server configuration
# can point these writes at a real database.
for name in $(env | sed -n 's/^\(\(BEADS\|BD\)_[A-Z_]*\)=.*/\1/p'); do
	unset "$name"
done
export BEADS_DIR="$work/.beads" BEADS_DOLT_AUTO_START=0 BD_NON_INTERACTIVE=1 BEADS_ACTOR=sam

# bd records the git identity as each issue's owner. Give it a made-up one
# rather than the identity of whoever runs this script.
printf '[user]\n\tname = sam\n\temail = sam@example.com\n' >"$work/gitconfig"
export GIT_CONFIG_GLOBAL="$work/gitconfig" GIT_CONFIG_NOSYSTEM=1

cd "$work"
bd init --prefix shop --quiet --skip-hooks >/dev/null
grep -q '"dolt_mode": "embedded"' .beads/metadata.json

# bd prints a hint about beads.role on stderr for every command.
new() { bd create --silent "$@" 2>/dev/null; }
# bd closes an assigned issue only for its assignee.
close_as() { who=$1; shift; BEADS_ACTOR=$who bd close "$@" >/dev/null 2>&1; }

checkout=$(new --type=epic --priority=1 --title="Checkout" --assignee=sam \
	--description="Let a visitor pay for the cart in one page.")
search=$(new --type=epic --priority=2 --title="Product search" --assignee=robin \
	--description="Find a product by name, brand or colour in under a second.")
launch=$(new --type=epic --priority=0 --title="Spring launch" --assignee=kim \
	--description="Everything the spring catalogue needs before it goes live.")
onboarding=$(new --type=epic --priority=3 --title="Seller onboarding" \
	--description="Sellers list their first product without help.")

cart=$(new --type=task --priority=1 --parent="$checkout" --assignee=sam --title="Cart summary with totals and VAT" \
	--labels=frontend --acceptance="Totals match the invoice to the cent.")
pay=$(new --type=feature --priority=1 --parent="$checkout" --assignee=alex --title="Card payments through the provider" \
	--labels=backend,payments --design="Redirect to the provider's hosted page. No card data touches our servers.")
webhook=$(new --type=task --priority=1 --parent="$checkout" --assignee=alex --title="Handle the payment webhook" \
	--labels=backend,payments)
receipt=$(new --type=task --priority=2 --parent="$checkout" --title="Email a receipt after payment" --labels=backend)
double=$(new --type=bug --priority=0 --parent="$checkout" --assignee=sam --title="Double click on Pay charges twice" \
	--labels=frontend,payments --description="Two quick clicks send two payment requests. Seen twice in the test shop.")

index=$(new --type=task --priority=2 --parent="$search" --assignee=robin --title="Build the search index nightly" --labels=backend)
typo=$(new --type=feature --priority=2 --parent="$search" --assignee=robin --title="Match misspelt product names" --labels=backend)
facets=$(new --type=feature --priority=3 --parent="$search" --title="Filter results by brand and colour" --labels=frontend)
slow=$(new --type=bug --priority=1 --parent="$search" --assignee=robin --title="Search takes 4 s on a cold cache" --labels=backend,performance)

photos=$(new --type=task --priority=1 --parent="$launch" --assignee=kim --title="Upload the spring catalogue photos" --labels=content)
banner=$(new --type=task --priority=2 --parent="$launch" --title="Homepage banner for spring" --labels=frontend,content)
prices=$(new --type=task --priority=1 --parent="$launch" --assignee=kim --title="Load the spring price list" --labels=content)
loadtest=$(new --type=task --priority=1 --parent="$launch" --assignee=alex --title="Load test checkout at 10x traffic" --labels=performance)

signup=$(new --type=task --priority=3 --parent="$onboarding" --title="Seller sign-up form")
guide=$(new --type=task --priority=4 --parent="$onboarding" --title="Write the seller guide" --labels=content)
logo=$(new --type=chore --priority=4 --title="Replace the favicon with the new logo" --labels=frontend)
deps=$(new --type=chore --priority=3 --assignee=alex --title="Update the web framework to the next minor release")

# Blocking work: the arrows the dependency graph and the blocked filter show.
bd dep add "$webhook" "$pay" >/dev/null
bd dep add "$receipt" "$webhook" >/dev/null
bd dep add "$loadtest" "$pay" >/dev/null
bd dep add "$launch" "$checkout" >/dev/null
bd dep add "$facets" "$index" >/dev/null
bd dep add "$typo" "$index" >/dev/null
bd dep add "$banner" "$photos" >/dev/null

bd update "$pay" --status=in_progress >/dev/null
bd update "$double" --status=in_progress >/dev/null
bd update "$slow" --status=in_progress >/dev/null
bd update "$photos" --status=in_progress >/dev/null
bd update "$loadtest" --status=blocked --notes="Waits for the payment provider's sandbox keys." >/dev/null
bd defer "$guide" --until=2027-03-01 >/dev/null

bd comments add "$double" -- "Reproduced on the staging shop with a slow network." >/dev/null
bd comments add "$double" -- "Disabling the button after the first click fixes it locally." >/dev/null
bd comments add "$pay" -- "Provider sandbox account requested." >/dev/null
bd comments add "$slow" -- "Most of the time goes to loading the synonym list." >/dev/null

close_as sam "$cart" --reason="Shipped"
close_as robin "$index" --reason="Runs at 02:00"
close_as kim "$prices" --reason="Loaded"
close_as sam "$logo" --reason="Done"
close_as sam "$signup" --reason="Built"

mkdir -p "$here/.beads"
bd export >"$here/.beads/issues.jsonl"
echo "wrote $(wc -l <"$here/.beads/issues.jsonl" | tr -d ' ') issues to $here/.beads/issues.jsonl"
